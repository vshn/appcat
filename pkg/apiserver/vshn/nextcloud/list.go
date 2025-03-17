package nextcloud

import (
	"context"
	"fmt"
	"time"

	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ rest.Lister = &vshnNextcloudBackupStorage{}

func (v *vshnNextcloudBackupStorage) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {

	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from resource")
	}

	instances, err := v.vshnNextcloud.ListVSHNnextcloud(ctx, namespace)
	if err != nil {
		return nil, err
	}

	nextcloudSnapshots := &appcatv1.VSHNNextcloudBackupList{
		Items: []appcatv1.VSHNNextcloudBackup{},
	}

	for _, instance := range instances.Items {
		client, err := v.vshnNextcloud.GetKubeClient(ctx, instance)
		if err != nil {
			return nil, fmt.Errorf("cannot get KubeClient from ProviderConfig")
		}
		snapshots, err := v.snapshothandler.List(ctx, instance.Status.InstanceNamespace, client)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}

		pgNamespace, pgCompName := v.getPostgreSQLNamespaceAndName(ctx, &instance)

		sgBackups := []appcatv1.SGBackupInfo{}
		if pgNamespace != "" {
			dynClient, err := v.vshnNextcloud.GetDynKubeClient(ctx, instance)
			if err != nil {
				return nil, err
			}
			sg, err := v.sgBackup.ListSGBackup(ctx, pgNamespace, dynClient, options)
			if err != nil {
				return nil, err
			}
			sgBackups = *sg
		}

		for _, snap := range snapshots.Items {

			backupMeta := snap.ObjectMeta
			backupMeta.Namespace = instance.GetNamespace()

			status := appcatv1.VSHNNextcloudBackupStatus{
				FileBackupAvailable: true,
				NextcloudFileBackup: appcatv1.VSHNNextcloudFileBackupStatus{
					ID:       deRefString(snap.Spec.ID),
					Date:     deRefMetaTime(snap.Spec.Date),
					Instance: instance.GetName(),
				},
			}

			sgBackup, remaining := v.getClosestPGBackup(sgBackups, *snap.Spec.Date)
			sgBackups = remaining

			if sgBackup != nil {
				status.DatabaseBackupStatus.BackupInformation = &sgBackup.BackupInformation
				status.DatabaseBackupStatus.Process = &sgBackup.Process
				status.DatabaseBackupStatus.DatabaseInstance = pgCompName
				status.DatabaseBackupAvailable = true
			}

			nextcloudSnapshots.Items = append(nextcloudSnapshots.Items, appcatv1.VSHNNextcloudBackup{
				ObjectMeta: backupMeta,
				Status:     status,
			})
		}

		// Let's also list all sgdbbackups without filebackups
		for _, sgBackup := range sgBackups {
			backupMeta := sgBackup.ObjectMeta

			nextcloudSnapshots.Items = append(nextcloudSnapshots.Items, appcatv1.VSHNNextcloudBackup{
				ObjectMeta: backupMeta,
				Status: appcatv1.VSHNNextcloudBackupStatus{
					DatabaseBackupAvailable: true,
					NextcloudFileBackup: appcatv1.VSHNNextcloudFileBackupStatus{
						ID:       hashString(backupMeta.GetName()),
						Instance: instance.GetName(),
						Date:     backupMeta.CreationTimestamp,
					},
					DatabaseBackupStatus: appcatv1.VSHNPostgresBackupStatus{
						BackupInformation: &sgBackup.BackupInformation,
						Process:           &sgBackup.Process,
						DatabaseInstance:  pgCompName,
					},
				},
			})
		}

	}

	return nextcloudSnapshots, nil
}

func (v *vshnNextcloudBackupStorage) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from resource")
	}

	instances, err := v.vshnNextcloud.ListVSHNnextcloud(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("cannot list VSHNPostgreSQL instances")
	}

	mw := apiserver.NewEmptyMultiWatch()
	for _, value := range instances.Items {
		client, err := v.vshnNextcloud.GetKubeClient(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("cannot get KubeClient from ProviderConfig")
		}
		backupWatcher, err := v.snapshothandler.Watch(ctx, value.Status.InstanceNamespace, options, client)
		if err != nil {
			return nil, apiserver.ResolveError(appcatv1.GetGroupResource(appcatv1.ResourceBackup), err)
		}
		mw.AddWatcher(backupWatcher)
	}

	return watch.Filter(mw, func(in watch.Event) (out watch.Event, keep bool) {
		if in.Object == nil {
			// This should never happen, let downstream deal with it
			return in, true
		}

		backupInfo, err := v.snapshothandler.GetFromEvent(in)
		if err != nil {
			return in, false
		}

		inst := ""
		namespace := ""
		for _, value := range instances.Items {
			if value.Status.InstanceNamespace == backupInfo.GetNamespace() {
				inst = value.GetName()
				namespace = value.GetNamespace()
			}
		}

		if inst == "" {
			return in, false
		}

		backupMeta := backupInfo.ObjectMeta
		backupMeta.Namespace = namespace

		in.Object = &appcatv1.VSHNNextcloudBackup{
			ObjectMeta: backupMeta,
			Status: appcatv1.VSHNNextcloudBackupStatus{
				FileBackupAvailable: true,
				NextcloudFileBackup: appcatv1.VSHNNextcloudFileBackupStatus{
					ID:       deRefString(backupInfo.Spec.ID),
					Date:     deRefMetaTime(backupInfo.Spec.Date),
					Instance: inst,
				},
			},
		}

		return in, true
	}), nil
}

func (v *vshnNextcloudBackupStorage) getPostgreSQLNamespaceAndName(ctx context.Context, inst *vshnv1.VSHNNextcloud) (string, string) {
	compName := inst.Spec.ResourceRef.Name
	ncComp := vshnv1.XVSHNNextcloud{}

	err := v.client.Get(ctx, client.ObjectKey{Name: compName}, &ncComp)
	if err != nil {
		return "", ""
	}

	pgName := ""
	for _, comp := range ncComp.Spec.ResourceRefs {
		if comp.Kind == "XVSHNPostgreSQL" {
			pgName = comp.Name
			break
		}
	}

	pgComp := &vshnv1.XVSHNPostgreSQL{}

	err = v.client.Get(ctx, client.ObjectKey{Name: pgName}, pgComp)
	if err != nil {
		return "", ""
	}

	return pgComp.Status.InstanceNamespace, pgName
}

// getClosestPGBackup gets the closest sgdbbackup to the given nextcloud backup, if it's available.
func (v *vshnNextcloudBackupStorage) getClosestPGBackup(backups []appcatv1.SGBackupInfo, ts metav1.Time) (*appcatv1.SGBackupInfo, []appcatv1.SGBackupInfo) {

	for i, backup := range backups {
		if backup.CreationTimestamp.Sub(ts.Time).Abs() <= 1*time.Minute {
			return &backup, append(backups[:i], backups[i+1:]...)
		}
	}

	return nil, backups
}

func (v *vshnNextcloudBackupStorage) NewList() runtime.Object {
	return &appcatv1.VSHNNextcloudBackupList{}
}
