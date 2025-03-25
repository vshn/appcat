package redis

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

var _ rest.Lister = &vshnRedisBackupStorage{}

func (v *vshnRedisBackupStorage) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from resource")
	}

	instances, err := v.vshnRedis.ListVSHNRedis(ctx, namespace)
	if err != nil {
		return nil, err
	}

	redisSnapshots := &appcatv1.VSHNRedisBackupList{
		Items: []appcatv1.VSHNRedisBackup{},
	}

	for _, instance := range instances.Items {
		client, err := v.vshnRedis.GetKubeClient(ctx, &instance)
		if err != nil {
			return nil, fmt.Errorf("cannot get KubeClient from ProviderConfig")
		}
		snapshots, err := v.snapshothandler.List(ctx, instance.Status.InstanceNamespace, client)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}

		for _, snap := range snapshots.Items {

			backupMeta := snap.ObjectMeta
			backupMeta.Namespace = instance.GetNamespace()

			redisSnapshots.Items = append(redisSnapshots.Items, appcatv1.VSHNRedisBackup{
				ObjectMeta: backupMeta,
				Status: appcatv1.VSHNRedisBackupStatus{
					ID:       deRefString(snap.Spec.ID),
					Date:     deRefMetaTime(snap.Spec.Date),
					Instance: instance.GetName(),
				},
			})
		}

	}

	return redisSnapshots, nil
}

func (v *vshnRedisBackupStorage) NewList() runtime.Object {
	return &appcatv1.VSHNRedisBackupList{}
}
