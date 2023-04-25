package postgres

import (
	"context"
	"fmt"
	"github.com/vshn/appcat-apiserver/apis/appcat/v1"
	"github.com/vshn/appcat-apiserver/apiserver"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Lister = &vshnPostgresBackupStorage{}

func (v *vshnPostgresBackupStorage) NewList() runtime.Object {
	return &v1.VSHNPostgresBackupList{}
}

// List returns a list of VSHNPostgresBackup services based on stackgres SGBackup resources
func (v *vshnPostgresBackupStorage) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from resource")
	}

	instances, err := v.vshnpostgresql.ListXVSHNPostgreSQL(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("cannot list VSHNPostgreSQL instances %v", err)
	}

	// Aggregate all backups from all postgres clusters from the same namespace
	backups := v1.VSHNPostgresBackupList{}
	for _, value := range instances.Items {
		bis, err := v.sgbackups.ListSGBackup(ctx, value.Status.InstanceNamespace, options)
		if err != nil {
			return nil, apiserver.ResolveError(v1.GetGroupResource(v1.ResourceBackup), err)
		}
		for _, b := range *bis {
			vb := v1.NewVSHNPostgresBackup(&b, value.Name, namespace)
			if vb != nil {
				backups.Items = append(backups.Items, *vb)
			}
		}
	}

	return &backups, nil
}

var _ rest.Watcher = &vshnPostgresBackupStorage{}

// Watch returns a watched list of VSHNPostgresBackup services based on stackgres SGBackup resources
func (v *vshnPostgresBackupStorage) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	namespace, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("cannot get namespace from resource")
	}

	instances, err := v.vshnpostgresql.ListXVSHNPostgreSQL(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("cannot list VSHNPostgreSQL instances")
	}

	mw := NewEmptyMultiWatch()
	for _, value := range instances.Items {
		backupWatcher, err := v.sgbackups.WatchSGBackup(ctx, value.Status.InstanceNamespace, options)
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

		sgbackupInfo := GetFromEvent(in)
		if sgbackupInfo == nil {
			return in, true
		}

		db := ""
		for _, value := range instances.Items {
			if value.Status.InstanceNamespace == sgbackupInfo.Namespace {
				db = value.Name
			}
		}

		if db == "" {
			return in, false
		}

		in.Object = v1.NewVSHNPostgresBackup(sgbackupInfo, db, namespace)

		if in.Object.(*v1.VSHNPostgresBackup) == nil {
			return in, false
		}

		return in, true
	}), nil
}
