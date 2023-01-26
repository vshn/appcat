package appcat

import (
	"apiserver/pkg/apis/appcat/v1"
	"k8s.io/apimachinery/pkg/runtime"
	genericregistry "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	restbuilder "sigs.k8s.io/apiserver-runtime/pkg/builder/rest"
	"sigs.k8s.io/apiserver-runtime/pkg/util/loopback"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New returns a new storage provider for AppCat
func New() restbuilder.ResourceHandlerProvider {
	return func(s *runtime.Scheme, gasdf genericregistry.RESTOptionsGetter) (rest.Storage, error) {
		c, err := client.NewWithWatch(loopback.GetLoopbackMasterClientConfig(), client.Options{})
		if err != nil {
			return nil, err
		}
		err = v1.AddToScheme(c.Scheme())
		if err != nil {
			return nil, err
		}
		return &appcatStorage{}, nil
	}
}

type appcatStorage struct{}

func (s appcatStorage) New() runtime.Object {
	return &v1.AppCat{}
}

var _ rest.Scoper = &appcatStorage{}

func (s *appcatStorage) NamespaceScoped() bool {
	return false
}
