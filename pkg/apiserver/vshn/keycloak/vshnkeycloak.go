package keycloak

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=vshnkeycloaks,verbs=get;list;watch
// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=xvshnkeycloaks,verbs=get;list;watch
// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=xvshnpostgresqls,verbs=get;list;watch

type vshnKeycloakProvider interface {
	ListVSHNkeycloak(ctx context.Context, namespace string) (*vshnv1.VSHNKeycloakList, error)
	apiserver.ClientConfigurator
}

type concreteKeycloakProvider struct {
	apiserver.ClientConfigurator
}

func (c *concreteKeycloakProvider) ListVSHNkeycloak(ctx context.Context, namespace string) (*vshnv1.VSHNKeycloakList, error) {

	instances := &vshnv1.VSHNKeycloakList{}

	err := c.List(ctx, instances, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, err
	}

	cleanedList := make([]vshnv1.VSHNKeycloak, 0)
	for _, p := range instances.Items {
		//
		// In some cases instance namespaces is missing and as a consequence all backups from the whole cluster
		// are being exposed creating a security issue - check APPCAT-563.
		if p.Status.InstanceNamespace != "" {
			cleanedList = append(cleanedList, p)
		}
	}
	instances.Items = cleanedList

	return instances, nil
}
