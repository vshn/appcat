package appcat

import (
	"apiserver/pkg/apis/appcat/v1"
	"context"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Lister = &appcatStorage{}

func (s appcatStorage) NewList() runtime.Object {
	return &v1.AppCatList{}
}

// List returns a one single test item
func (s *appcatStorage) List(_ context.Context, _ *metainternalversion.ListOptions) (runtime.Object, error) {
	return &v1.AppCatList{
		Items: []v1.AppCat{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "appcat-1",
					CreationTimestamp: metav1.Now(),
				},
				Spec: v1.AppCatSpec{
					ServiceName: "test-service-1",
				},
				Status: v1.AppCatStatus{},
			},
		},
	}, nil
}

var _ rest.Watcher = &appcatStorage{}

func (s *appcatStorage) Watch(_ context.Context, _ *metainternalversion.ListOptions) (watch.Interface, error) {
	return watch.NewEmptyWatch(), nil
}
