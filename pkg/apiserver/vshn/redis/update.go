package redis

import (
	"context"
	"fmt"

	v1 "github.com/vshn/appcat/v4/apis/appcat/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Updater = &vshnRedisBackupStorage{}
var _ rest.CreaterUpdater = &vshnRedisBackupStorage{}

func (v vshnRedisBackupStorage) Update(_ context.Context, name string, _ rest.UpdatedObjectInfo, _ rest.ValidateObjectFunc, _ rest.ValidateObjectUpdateFunc, _ bool, _ *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return &v1.VSHNPostgresBackup{}, false, fmt.Errorf("method not implemented")
}
