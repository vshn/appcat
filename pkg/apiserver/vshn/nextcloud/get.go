package nextcloud

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
		ins := instance.Status.InstanceNamespace
		snap, err := v.snapshothandler.Get(ctx, name, ins)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}

		backupMeta := snap.ObjectMeta
		backupMeta.Namespace = instance.GetNamespace()

		nextcloudSnap = &appcatv1.VSHNNextcloudBackup{
			ObjectMeta: backupMeta,
			Status: appcatv1.VSHNNextcloudBackupStatus{
				ID:       deRefString(snap.Spec.ID),
				Date:     deRefMetaTime(snap.Spec.Date),
				Instance: instance.GetName(),
			},
		}
	}

	return nextcloudSnap, nil
}
