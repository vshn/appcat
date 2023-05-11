package postgres

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Creater = &vshnPostgresBackupStorage{}

func (v vshnPostgresBackupStorage) Create(_ context.Context, _ runtime.Object, _ rest.ValidateObjectFunc, _ *metav1.CreateOptions) (runtime.Object, error) {
	return nil, fmt.Errorf("method not implemented")
}
