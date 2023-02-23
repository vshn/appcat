package postgres

import (
	"appcat-apiserver/apis/appcat/v1"
	"context"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.GracefulDeleter = &vshnPostgresBackupStorage{}
var _ rest.CollectionDeleter = &vshnPostgresBackupStorage{}

func (v vshnPostgresBackupStorage) Delete(_ context.Context, name string, _ rest.ValidateObjectFunc, _ *metav1.DeleteOptions) (runtime.Object, bool, error) {
	return &v1.VSHNPostgresBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}, false, nil
}

func (v *vshnPostgresBackupStorage) DeleteCollection(_ context.Context, _ rest.ValidateObjectFunc, _ *metav1.DeleteOptions, _ *metainternalversion.ListOptions) (runtime.Object, error) {
	return &v1.VSHNPostgresBackupList{
		Items: []v1.VSHNPostgresBackup{},
	}, nil
}
