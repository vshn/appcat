package redis

import (
	"context"
	"fmt"

	v1 "github.com/vshn/appcat/apis/appcat/v1"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.GracefulDeleter = &vshnRedisBackupStorage{}
var _ rest.CollectionDeleter = &vshnRedisBackupStorage{}

func (v vshnRedisBackupStorage) Delete(_ context.Context, name string, _ rest.ValidateObjectFunc, _ *metav1.DeleteOptions) (runtime.Object, bool, error) {
	return &v1.VSHNPostgresBackup{}, false, fmt.Errorf("method not implemented")
}

func (v *vshnRedisBackupStorage) DeleteCollection(ctx context.Context, _ rest.ValidateObjectFunc, _ *metav1.DeleteOptions, _ *metainternalversion.ListOptions) (runtime.Object, error) {
	return &v1.VSHNPostgresBackupList{
		Items: []v1.VSHNPostgresBackup{},
	}, fmt.Errorf("method not implemented")
}
