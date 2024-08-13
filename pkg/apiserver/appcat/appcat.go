package appcat

import (
	crossplane "github.com/crossplane/crossplane/apis/apiextensions/v1"
	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	"github.com/vshn/appcat/v4/pkg/apiserver/noop"
	"k8s.io/apimachinery/pkg/runtime"
	genericregistry "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	restbuilder "sigs.k8s.io/apiserver-runtime/pkg/builder/rest"
	"sigs.k8s.io/apiserver-runtime/pkg/util/loopback"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch,resourceNames=extension-apiserver-authentication
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;delete;update
// +kubebuilder:rbac:groups="authorization.k8s.io",resources=subjectaccessreviews,verbs=get;list;watch;create;delete;update
// +kubebuilder:rbac:groups="apiextensions.crossplane.io",resources=compositions,verbs=get;list;watch

// New returns a new storage provider for AppCat
func New() restbuilder.ResourceHandlerProvider {
	return func(s *runtime.Scheme, gasdf genericregistry.RESTOptionsGetter) (rest.Storage, error) {
		c, err := client.NewWithWatch(loopback.GetLoopbackMasterClientConfig(), client.Options{})
		if err != nil {
			return nil, err
		}
		err = appcatv1.AddToScheme(c.Scheme())
		if err != nil {
			return nil, err
		}
		err = crossplane.AddToScheme(c.Scheme())
		if err != nil {
			return nil, err
		}

		noopImplementation := noop.New(s, &appcatv1.AppCat{}, &appcatv1.AppCatList{})

		if !apiserver.IsTypeAvailable(crossplane.SchemeGroupVersion.String(), "Composition") {
			return noopImplementation, nil
		}

		return &appcatStorage{
			compositions: &kubeCompositionProvider{
				Client: c,
			},
			Noop: *noopImplementation,
		}, nil
	}
}

type appcatStorage struct {
	compositions compositionProvider
	noop.Noop
}

// GetSingularName is needed for the OpenAPI Registartion
func (in *appcatStorage) GetSingularName() string {
	return "appcat"
}

func (s *appcatStorage) New() runtime.Object {
	return &appcatv1.AppCat{}
}

func (s *appcatStorage) Destroy() {}

var _ rest.Scoper = &appcatStorage{}
var _ rest.Storage = &appcatStorage{}

func (s *appcatStorage) NamespaceScoped() bool {
	return false
}
