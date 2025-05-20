package nextcloud

import (
	"context"
	"fmt"

	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Getter = &vshnNextcloudBackupStorage{}

func (v *vshnNextcloudBackupStorage) Get(ctx context.Context, name string, _ *metav1.GetOptions) (runtime.Object, error) {

	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from context")
	}

	instances, err := v.vshnNextcloud.ListVSHNnextcloud(ctx, namespace)
	if err != nil {
		return nil, err
	}

	nextcloudSnap := &appcatv1.VSHNNextcloudBackup{}

	for _, instance := range instances.Items {
		client, err := v.vshnNextcloud.GetKubeClient(ctx, &instance)
		if err != nil {
			return nil, fmt.Errorf("cannot get KubeClient from ProviderConfig: %w", err)
		}
		ins := instance.Status.InstanceNamespace
		snap, err := v.snapshothandler.Get(ctx, name, ins, client)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
		}

		status := appcatv1.VSHNNextcloudBackupStatus{}
		var sgBackup *appcatv1.SGBackupInfo
		backupMeta := metav1.ObjectMeta{}
		pgNamespace, pgName := v.getPostgreSQLNamespaceAndName(ctx, &instance)
		dynClient, err := v.vshnNextcloud.GetDynKubeClient(ctx, &instance)
		if err != nil {
			return nil, err
		}
		sgBackups, err := v.sgBackup.ListSGBackup(ctx, pgNamespace, dynClient, &internalversion.ListOptions{})
		if err != nil {
			return nil, err
		}

		if snap != nil {
			status.FileBackupAvailable = true
			status.NextcloudFileBackup = appcatv1.VSHNNextcloudFileBackupStatus{
				ID:       deRefString(snap.Spec.ID),
				Date:     deRefMetaTime(snap.Spec.Date),
				Instance: instance.GetName(),
			}
			sgBackup, _ = v.getClosestPGBackup(*sgBackups, *snap.Spec.Date)
			backupMeta = snap.ObjectMeta
		}

		if sgBackup == nil {
			for _, back := range *sgBackups {
				hashedName := hashString(back.GetName())
				if name == hashedName {
					sgBackup = &back
					break
				}
			}
		}

		if sgBackup != nil {
			status.DatabaseBackupAvailable = true
			status.DatabaseBackupStatus = appcatv1.VSHNPostgresBackupStatus{
				BackupInformation: &sgBackup.BackupInformation,
				Process:           &sgBackup.Process,
				DatabaseInstance:  pgName,
			}
			if snap == nil {
				status.NextcloudFileBackup = appcatv1.VSHNNextcloudFileBackupStatus{
					ID:       hashString(sgBackup.GetName()),
					Date:     sgBackup.CreationTimestamp,
					Instance: instance.GetName(),
				}
				backupMeta = sgBackup.ObjectMeta
			}
		}

		backupMeta.Namespace = instance.GetNamespace()

		nextcloudSnap = &appcatv1.VSHNNextcloudBackup{
			ObjectMeta: backupMeta,
			Status:     status,
		}
	}

	return nextcloudSnap, nil
}
