package mariadb

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=vshnmariadbs,verbs=get;list;watch

type vshnMariaDBProvider interface {
	ListVSHNMariaDB(ctx context.Context, namespace string) (*vshnv1.VSHNMariaDBList, error)
	apiserver.ClientConfigurator
}

type concreteMariaDBProvider struct {
	apiserver.ClientConfigurator
}

func (c *concreteMariaDBProvider) ListVSHNMariaDB(ctx context.Context, namespace string) (*vshnv1.VSHNMariaDBList, error) {

	instances := &vshnv1.VSHNMariaDBList{}

	err := c.List(ctx, instances, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, err
	}

	cleanedList := make([]vshnv1.VSHNMariaDB, 0)
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
