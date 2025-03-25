package redis

import (
	"context"
	"fmt"

	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Getter = &vshnRedisBackupStorage{}

func (v *vshnRedisBackupStorage) Get(ctx context.Context, name string, opts *metav1.GetOptions) (runtime.Object, error) {

	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from context")
	}

	instances, err := v.vshnRedis.ListVSHNRedis(ctx, namespace)
	if err != nil {
		return nil, err
	}

	redisSnap := &appcatv1.VSHNRedisBackup{}

	for _, instance := range instances.Items {
		client, err := v.vshnRedis.GetKubeClient(ctx, &instance)
		if err != nil {
			return nil, fmt.Errorf("cannot get KubeClient from ProviderConfig")
		}
		ins := instance.Status.InstanceNamespace
		snap, err := v.snapshothandler.Get(ctx, name, ins, client)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}

		backupMeta := snap.ObjectMeta
		backupMeta.Namespace = instance.GetNamespace()

		redisSnap = &appcatv1.VSHNRedisBackup{
			ObjectMeta: backupMeta,
			Status: appcatv1.VSHNRedisBackupStatus{
				ID:       deRefString(snap.Spec.ID),
				Date:     deRefMetaTime(snap.Spec.Date),
				Instance: instance.GetName(),
			},
		}
	}

	return redisSnap, nil
}
