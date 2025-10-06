package nextcloud

import (
	"context"
	"fmt"
	"time"

	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	"github.com/vshn/appcat/v4/pkg/apiserver/vshn/postgres"
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
		return nil, fmt.Errorf("cannot get namespace from context")
	}

	instances, err := v.vshnNextcloud.ListVSHNnextcloud(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("cannot list VSHNNextcloud instances: %w", err)
	}

	nextcloudSnapshots := &appcatv1.VSHNNextcloudBackupList{}

	for _, instance := range instances.Items {
		client, err := v.vshnNextcloud.GetKubeClient(ctx, &instance)
		if err != nil {
			return nil, fmt.Errorf("cannot get kube client from ProviderConfig: %w", err)
		}

		snapshots, err := v.snapshothandler.List(ctx, instance.Status.InstanceNamespace, client)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("error listing file backups: %w", err)
		}

		pgNamespace, pgCompName, pgCompRefName := v.getPostgreSQLMetadata(ctx, &instance)

		var backups []appcatv1.BackupInfo
		if pgNamespace != "" {
			dynClient, err := v.vshnNextcloud.GetDynKubeClient(ctx, &instance)
			if err != nil {
				return nil, fmt.Errorf("cannot get dynamic kube client: %w", err)
			}

			schema := postgres.DetermineTargetSchema(pgCompRefName)
			backupList, err := v.backup.ListBackup(ctx, pgNamespace, schema, dynClient, options)
			if err != nil {
				return nil, fmt.Errorf("error listing postgres backups: %w", err)
			}
			backups = *backupList
		}

		isSharedDB := instance.Spec.Parameters.Service.UseExternalPostgreSQL &&
			instance.Spec.Parameters.Service.ExistingPGConnectionSecret != ""

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

			if isSharedDB {
				status.DBType = appcatv1.Shared
				status.DatabaseBackupAvailable = false
			} else {
				status.DBType = appcatv1.Dedicated
				if backup, remaining := v.getClosestPGBackup(backups, *snap.Spec.Date); backup != nil {
					status.DatabaseBackupAvailable = true
					status.DatabaseBackupStatus = appcatv1.VSHNPostgresBackupStatus{
						BackupInformation: &backup.BackupInformation,
						Process:           &backup.Process,
						DatabaseInstance:  pgCompName,
					}
					backups = remaining
				}
			}

			nextcloudSnapshots.Items = append(nextcloudSnapshots.Items, appcatv1.VSHNNextcloudBackup{
				ObjectMeta: backupMeta,
				Status:     status,
			})
		}

		// For external PostgreSQL we skip listing remaining SG backups
		if isSharedDB {
			continue
		}

		// Include remaining PostgreSQL backups that had no matching file backup
		for _, backup := range backups {
			backupMeta := backup.ObjectMeta

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
						BackupInformation: &backup.BackupInformation,
						Process:           &backup.Process,
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
		client, err := v.vshnNextcloud.GetKubeClient(ctx, &value)
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

// Get namespace, name and compositionReference (in that order) of a XVSHNPostgreSQL
func (v *vshnNextcloudBackupStorage) getPostgreSQLMetadata(ctx context.Context, inst *vshnv1.VSHNNextcloud) (string, string, string) {
	compName := inst.Spec.ResourceRef.Name
	ncComp := vshnv1.XVSHNNextcloud{}

	err := v.vshnNextcloud.Get(ctx, client.ObjectKey{Name: compName}, &ncComp)
	if err != nil {
		return "", "", ""
	}

	pgName := ""
	for _, comp := range ncComp.Spec.ResourceRefs {
		if comp.Kind == "XVSHNPostgreSQL" {
			pgName = comp.Name
			break
		}
	}

	pgComp := &vshnv1.XVSHNPostgreSQL{}

	err = v.vshnNextcloud.Get(ctx, client.ObjectKey{Name: pgName}, pgComp)
	if err != nil {
		return "", "", ""
	}

	return pgComp.Status.InstanceNamespace, pgName, pgComp.Spec.CompositionRef.Name
}

// getClosestPGBackup gets the closest sgdbbackup to the given nextcloud backup, if it's available.
func (v *vshnNextcloudBackupStorage) getClosestPGBackup(backups []appcatv1.BackupInfo, ts metav1.Time) (*appcatv1.BackupInfo, []appcatv1.BackupInfo) {

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
