package nextcloud

import (
	"context"
	"fmt"
	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Getter = &vshnNextcloudBackupStorage{}

func (v *vshnNextcloudBackupStorage) Get(ctx context.Context, name string, _ *metav1.GetOptions) (runtime.Object, error) {
	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot extract namespace from context")
	}

	instances, err := v.vshnNextcloud.ListVSHNnextcloud(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("cannot list VSHNNextcloud instances: %w", err)
	}

	for _, instance := range instances.Items {
		client, err := v.vshnNextcloud.GetKubeClient(ctx, &instance)
		if err != nil {
			return nil, fmt.Errorf("cannot get kube client for instance %s: %w", instance.Name, err)
		}

		instanceNS := instance.Status.InstanceNamespace
		snap, err := v.snapshothandler.Get(ctx, name, instanceNS, client)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("error getting file snapshot: %w", err)
		}

		var (
			status     = appcatv1.VSHNNextcloudBackupStatus{}
			backupMeta metav1.ObjectMeta
			snapFound  = snap != nil
		)

		if snapFound {
			status.FileBackupAvailable = true
			status.NextcloudFileBackup = appcatv1.VSHNNextcloudFileBackupStatus{
				ID:       deRefString(snap.Spec.ID),
				Date:     deRefMetaTime(snap.Spec.Date),
				Instance: instance.Name,
			}
			backupMeta = snap.ObjectMeta
		}

		isSharedPG := instance.Spec.Parameters.Service.UseExternalPostgreSQL &&
			instance.Spec.Parameters.Service.ExistingVSHNPostgreSQLConnectionSecret != ""

		if isSharedPG {
			status.DBType = appcatv1.Shared
			status.DatabaseBackupAvailable = false
			backupMeta.Namespace = instance.Namespace

			return &appcatv1.VSHNNextcloudBackup{
				ObjectMeta: backupMeta,
				Status:     status,
			}, nil
		}

		// Continue searching for database backup
		status.DBType = appcatv1.Dedicated
		pgNamespace, pgCompName := v.getPostgreSQLNamespaceAndName(ctx, &instance)

		dynClient, err := v.vshnNextcloud.GetDynKubeClient(ctx, &instance)
		if err != nil {
			return nil, fmt.Errorf("cannot get dynamic kube client: %w", err)
		}

		sgBackups, err := v.sgBackup.ListSGBackup(ctx, pgNamespace, dynClient, &internalversion.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("cannot list SGBackups: %w", err)
		}

		var matchedBackup *appcatv1.SGBackupInfo
		if snapFound {
			matchedBackup, _ = v.getClosestPGBackup(*sgBackups, *snap.Spec.Date)
		}

		if matchedBackup == nil {
			for _, b := range *sgBackups {
				if name == hashString(b.GetName()) {
					matchedBackup = &b
					break
				}
			}
		}

		if matchedBackup != nil {
			status.DatabaseBackupAvailable = true
			status.DatabaseBackupStatus = appcatv1.VSHNPostgresBackupStatus{
				BackupInformation: &matchedBackup.BackupInformation,
				Process:           &matchedBackup.Process,
				DatabaseInstance:  pgCompName,
			}
			if !snapFound {
				status.NextcloudFileBackup = appcatv1.VSHNNextcloudFileBackupStatus{
					ID:       hashString(matchedBackup.GetName()),
					Date:     matchedBackup.CreationTimestamp,
					Instance: instance.Name,
				}
				backupMeta = matchedBackup.ObjectMeta
			}
		}

		backupMeta.Namespace = instance.Namespace
		return &appcatv1.VSHNNextcloudBackup{
			ObjectMeta: backupMeta,
			Status:     status,
		}, nil
	}

	return nil, apierrors.NewNotFound(schema.GroupResource{Group: "appcat.vshn.io", Resource: "vshnnextcloudbackup"}, name)
}
