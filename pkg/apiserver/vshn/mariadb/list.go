package mariadb

import (
	"context"
	"fmt"

	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Lister = &vshnMariaDBBackupStorage{}

func (v *vshnMariaDBBackupStorage) List(ctx context.Context, _ *metainternalversion.ListOptions) (runtime.Object, error) {

	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from resource")
	}

	instances, err := v.vshnMariaDB.ListVSHNMariaDB(ctx, namespace)
	if err != nil {
		return nil, err
	}

	mariadbSnapshots := &appcatv1.VSHNMariaDBBackupList{
		Items: []appcatv1.VSHNMariaDBBackup{},
	}

	for _, instance := range instances.Items {
		client, err := v.vshnMariaDB.GetKubeClient(ctx, &instance)
		if err != nil {
			return nil, fmt.Errorf("cannot get KubeClient from ProviderConfig: %w", err)
		}
		snapshots, err := v.snapshothandler.List(ctx, instance.Status.InstanceNamespace, client)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}

		for _, snap := range snapshots.Items {

			backupMeta := snap.ObjectMeta
			backupMeta.Namespace = instance.GetNamespace()

			mariadbSnapshots.Items = append(mariadbSnapshots.Items, appcatv1.VSHNMariaDBBackup{
				ObjectMeta: backupMeta,
				Status: appcatv1.VSHNMariaDBBackupStatus{
					ID:       deRefString(snap.Spec.ID),
					Date:     deRefMetaTime(snap.Spec.Date),
					Instance: instance.GetName(),
				},
			})
		}

	}

	return mariadbSnapshots, nil
}

func (v *vshnMariaDBBackupStorage) NewList() runtime.Object {
	return &appcatv1.VSHNMariaDBBackupList{}
}
