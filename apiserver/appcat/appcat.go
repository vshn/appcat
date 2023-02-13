package appcat

import (
	v1 "appcat-apiserver/apis/appcat/v1"
	"errors"
	crossplane "github.com/crossplane/crossplane/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		err = crossplane.AddToScheme(c.Scheme())
		if err != nil {
			return nil, err
		}
		return &appcatStorage{
			compositions: &kubeCompositionProvider{
				Client: c,
			},
		}, nil
	}
}

type appcatStorage struct {
	compositions compositionProvider
}

func (s appcatStorage) New() runtime.Object {
	return &v1.AppCat{}
}

func (s appcatStorage) Destroy() {}

var _ rest.Scoper = &appcatStorage{}
var _ rest.Storage = &appcatStorage{}

func (s *appcatStorage) NamespaceScoped() bool {
	return false
}

func convertCompositionError(err error) error {
	groupResource := schema.GroupResource{
		Group:    v1.GroupVersion.Group,
		Resource: "appcats",
	}
	statusErr := &apierrors.StatusError{}

	if errors.As(err, &statusErr) {
		switch {
		case apierrors.IsNotFound(err):
			return apierrors.NewNotFound(groupResource, statusErr.ErrStatus.Details.Name)
		case apierrors.IsAlreadyExists(err):
			return apierrors.NewAlreadyExists(groupResource, statusErr.ErrStatus.Details.Name)
		}
	}
	return err
}
