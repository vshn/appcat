package keycloak

import (
	"context"
	"fmt"

	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	"github.com/vshn/appcat/v4/pkg/apiserver/vshn/postgres"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Lister = &vshnKeycloakBackupStorage{}

func (v *vshnKeycloakBackupStorage) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {

	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from resource")
	}

	instances, err := v.vshnKeycloak.ListVSHNkeycloak(ctx, namespace)
	if err != nil {
		return nil, err
	}

	keycloakSnapshots := &appcatv1.VSHNKeycloakBackupList{
		Items: []appcatv1.VSHNKeycloakBackup{},
	}

	for _, instance := range instances.Items {
		pgNamespace, pgCompName := v.getPostgreSQLNamespaceAndName(ctx, &instance)

		sgBackups := []appcatv1.SGBackupInfo{}
		if pgNamespace != "" {
			dynClient, err := v.vshnKeycloak.GetDynKubeClient(ctx, &instance)
			if err != nil {
				return nil, err
			}
			sg, err := v.sgBackup.ListSGBackup(ctx, pgNamespace, dynClient, options)
			if err != nil {
				return nil, err
			}
			sgBackups = *sg
		}

		for _, sgBackup := range sgBackups {
			backupMeta := sgBackup.ObjectMeta
			backupMeta.Namespace = instance.GetNamespace()

			keycloakSnapshots.Items = append(keycloakSnapshots.Items, appcatv1.VSHNKeycloakBackup{
				ObjectMeta: backupMeta,
				Status: appcatv1.VSHNKeycloakBackupStatus{
					DatabaseBackupAvailable: true,
					DatabaseBackupStatus: appcatv1.VSHNPostgresBackupStatus{
						BackupInformation: &sgBackup.BackupInformation,
						Process:           &sgBackup.Process,
						DatabaseInstance:  pgCompName,
					},
				},
			})
		}
	}

	return keycloakSnapshots, nil
}

func (v *vshnKeycloakBackupStorage) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from resource")
	}

	instances, err := v.vshnKeycloak.ListVSHNkeycloak(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("cannot list VSHNKeycloak instances: %w", err)
	}

	mw := apiserver.NewEmptyMultiWatch()
	for _, instance := range instances.Items {
		pgNamespace, _ := v.getPostgreSQLNamespaceAndName(ctx, &instance)
		if pgNamespace == "" {
			continue // Skip if no PostgreSQL instance is found
		}

		backupWatcher, err := v.sgBackup.WatchSGBackup(ctx, pgNamespace, options)
		if err != nil {
			return nil, apiserver.ResolveError(appcatv1.GetGroupResource(appcatv1.ResourceBackup), err)
		}
		mw.AddWatcher(backupWatcher)
	}

	return watch.Filter(mw, func(in watch.Event) (out watch.Event, keep bool) {
		if in.Object == nil {
			return in, true
		}

		backupInfo := postgres.GetFromEvent(in)
		if backupInfo == nil {
			return in, false
		}

		var matchedInstance *vshnv1.VSHNKeycloak
		for i := range instances.Items {
			if instances.Items[i].Status.InstanceNamespace == backupInfo.GetNamespace() {
				matchedInstance = &instances.Items[i]
				break
			}
		}

		if matchedInstance == nil {
			return in, false
		}

		_, pgName := v.getPostgreSQLNamespaceAndName(ctx, matchedInstance)

		backupMeta := backupInfo.ObjectMeta
		backupMeta.Namespace = matchedInstance.GetNamespace()

		in.Object = &appcatv1.VSHNKeycloakBackup{
			ObjectMeta: backupMeta,
			Status: appcatv1.VSHNKeycloakBackupStatus{
				DatabaseBackupAvailable: true,
				DatabaseBackupStatus: appcatv1.VSHNPostgresBackupStatus{
					Process:           &backupInfo.Process,
					BackupInformation: &backupInfo.BackupInformation,
					DatabaseInstance:  pgName,
				},
			},
		}

		return in, true
	}), nil
}

func (v *vshnKeycloakBackupStorage) getPostgreSQLNamespaceAndName(ctx context.Context, inst *vshnv1.VSHNKeycloak) (string, string) {
	pgCompName := inst.Status.InstanceNamespace
	if pgCompName == "" {
		return "", ""
	}

	return inst.Status.InstanceNamespace, pgCompName
}

func (v *vshnKeycloakBackupStorage) NewList() runtime.Object {
	return &appcatv1.VSHNKeycloakBackupList{}
}
