package mariadb

import (
	"context"
	"fmt"

	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	v1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Watcher = &vshnMariaDBBackupStorage{}

func (v *vshnMariaDBBackupStorage) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from resource")
	}

	instances, err := v.vshnMariaDB.ListVSHNMariaDB(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("cannot list VSHNPostgreSQL instances")
	}

	mw := apiserver.NewEmptyMultiWatch()
	for _, value := range instances.Items {
		client, err := v.vshnMariaDB.GetKubeClient(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("cannot get KubeClient from ProviderConfig")
		}
		backupWatcher, err := v.snapshothandler.Watch(ctx, value.Status.InstanceNamespace, options, client)
		if err != nil {
			return nil, apiserver.ResolveError(v1.GetGroupResource(v1.ResourceBackup), err)
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

		in.Object = &appcatv1.VSHNMariaDBBackup{
			ObjectMeta: backupMeta,
			Status: v1.VSHNMariaDBBackupStatus{
				ID:       deRefString(backupInfo.Spec.ID),
				Date:     deRefMetaTime(backupInfo.Spec.Date),
				Instance: db,
			},
		}

		return in, true
	}), nil
}
