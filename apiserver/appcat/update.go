package appcat

import (
	v1 "appcat-apiserver/apis/appcat/v1"
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Updater = &appcatStorage{}
var _ rest.CreaterUpdater = &appcatStorage{}

func (s appcatStorage) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return &v1.AppCat{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}, false, nil
}
