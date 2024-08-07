package nextcloud

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

var _ rest.Lister = &vshnNextcloudBackupStorage{}

func (v *vshnNextcloudBackupStorage) List(ctx context.Context, _ *metainternalversion.ListOptions) (runtime.Object, error) {

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
		snapshots, err := v.snapshothandler.List(ctx, instance.Status.InstanceNamespace)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}

		for _, snap := range snapshots.Items {

			backupMeta := snap.ObjectMeta
			backupMeta.Namespace = instance.GetNamespace()

			nextcloudSnapshots.Items = append(nextcloudSnapshots.Items, appcatv1.VSHNNextcloudBackup{
				ObjectMeta: backupMeta,
				Status: appcatv1.VSHNNextcloudBackupStatus{
					ID:       deRefString(snap.Spec.ID),
					Date:     deRefMetaTime(snap.Spec.Date),
					Instance: instance.GetName(),
				},
			})
		}

	}

	return nextcloudSnapshots, nil
}

func (v *vshnNextcloudBackupStorage) NewList() runtime.Object {
	return &appcatv1.VSHNNextcloudBackupList{}
}
