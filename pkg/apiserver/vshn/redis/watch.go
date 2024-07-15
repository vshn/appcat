package redis

import (
	"context"
	"fmt"

	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Watcher = &vshnRedisBackupStorage{}

func (v *vshnRedisBackupStorage) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from resource")
	}

	instances, err := v.vshnRedis.ListVSHNRedis(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("cannot list VSHNPostgreSQL instances")
	}

	mw := apiserver.NewEmptyMultiWatch()
	for _, value := range instances.Items {
		backupWatcher, err := v.snapshothandler.Watch(ctx, value.Status.InstanceNamespace, options)
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

		db := ""
		namespace := ""
		for _, value := range instances.Items {
			if value.Status.InstanceNamespace == backupInfo.GetNamespace() {
				db = value.GetName()
				namespace = value.GetNamespace()
			}
		}

		if db == "" {
			return in, false
		}

		backupMeta := backupInfo.ObjectMeta
		backupMeta.Namespace = namespace

		in.Object = &appcatv1.VSHNRedisBackup{
			ObjectMeta: backupMeta,
			Status: appcatv1.VSHNRedisBackupStatus{
				ID:       deRefString(backupInfo.Spec.ID),
				Date:     deRefMetaTime(backupInfo.Spec.Date),
				Instance: db,
			},
		}

		return in, true
	}), nil
}
