package postgres

import (
	"context"
	"github.com/vshn/appcat-apiserver/apis/appcat/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Updater = &vshnPostgresBackupStorage{}
var _ rest.CreaterUpdater = &vshnPostgresBackupStorage{}

func (v vshnPostgresBackupStorage) Update(_ context.Context, name string, _ rest.UpdatedObjectInfo, _ rest.ValidateObjectFunc, _ rest.ValidateObjectUpdateFunc, _ bool, _ *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return &v1.VSHNPostgresBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}, false, nil
}
