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
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		pgNamespace, pgCompName, pgCompRefName := v.getPostgreSQLMetadata(ctx, &instance)

		backups := []appcatv1.BackupInfo{}
		if pgNamespace != "" {
			dynClient, err := v.vshnKeycloak.GetDynKubeClient(ctx, &instance)
			if err != nil {
				return nil, err
			}

			schema := postgres.DetermineTargetSchema(pgCompRefName)
			b, err := v.backup.ListBackup(ctx, pgNamespace, schema, dynClient, options)
			if err != nil {
				return nil, err
			}
			backups = *b
		}

		for _, backup := range backups {
			backupMeta := backup.ObjectMeta
			backupMeta.Namespace = instance.GetNamespace()

			keycloakSnapshots.Items = append(keycloakSnapshots.Items, appcatv1.VSHNKeycloakBackup{
				ObjectMeta: backupMeta,
				Status: appcatv1.VSHNKeycloakBackupStatus{
					DatabaseBackupAvailable: true,
					DatabaseBackupStatus: appcatv1.VSHNPostgresBackupStatus{
						BackupInformation: &backup.BackupInformation,
						Process:           &backup.Process,
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
		pgNamespace, _, _ := v.getPostgreSQLMetadata(ctx, &instance)
		if pgNamespace == "" {
			continue // Skip if no PostgreSQL instance is found
		}

		backupWatcher, err := v.backup.WatchBackup(ctx, pgNamespace, options)
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

		_, pgName, _ := v.getPostgreSQLMetadata(ctx, matchedInstance)

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

// Get namespace, name and compositionReference (in that order) of a XVSHNPostgreSQL
func (v *vshnKeycloakBackupStorage) getPostgreSQLMetadata(ctx context.Context, inst *vshnv1.VSHNKeycloak) (string, string, string) {
	compName := inst.Spec.ResourceRef.Name
	kcComp := vshnv1.XVSHNKeycloak{}

	err := v.vshnKeycloak.Get(ctx, client.ObjectKey{Name: compName}, &kcComp)
	if err != nil {
		return "", "", ""
	}

	pgName := ""
	for _, comp := range kcComp.Spec.ResourceRefs {
		if comp.Kind == "XVSHNPostgreSQL" {
			pgName = comp.Name
			break
		}
	}

	pgComp := &vshnv1.XVSHNPostgreSQL{}

	err = v.vshnKeycloak.Get(ctx, client.ObjectKey{Name: pgName}, pgComp)
	if err != nil {
		return "", "", ""
	}

	return pgComp.Status.InstanceNamespace, pgName, pgComp.Spec.CompositionRef.Name
}

func (v *vshnKeycloakBackupStorage) NewList() runtime.Object {
	return &appcatv1.VSHNKeycloakBackupList{}
}
