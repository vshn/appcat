package appcat

import (
	"context"
	v1 "github.com/vshn/appcat-apiserver/apis/appcat/v1"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.GracefulDeleter = &appcatStorage{}
var _ rest.CollectionDeleter = &appcatStorage{}

func (s *appcatStorage) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	return &v1.AppCat{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}, false, nil
}

func (s *appcatStorage) DeleteCollection(ctx context.Context, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions, listOptions *metainternalversion.ListOptions) (runtime.Object, error) {
	return &v1.AppCatList{
		Items: []v1.AppCat{},
	}, nil
}
