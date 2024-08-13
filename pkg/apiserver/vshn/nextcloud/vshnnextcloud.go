package nextcloud

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=vshnnextclouds,verbs=get;list;watch
// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=xvshnnextclouds,verbs=get;list;watch
// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=xvshnpostgresqls,verbs=get;list;watch

type vshnNextcloudProvider interface {
	ListVSHNnextcloud(ctx context.Context, namespace string) (*vshnv1.VSHNNextcloudList, error)
}

type concreteNextcloudProvider struct {
	client client.Client
}

func (c *concreteNextcloudProvider) ListVSHNnextcloud(ctx context.Context, namespace string) (*vshnv1.VSHNNextcloudList, error) {

	instances := &vshnv1.VSHNNextcloudList{}

	err := c.client.List(ctx, instances, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, err
	}

	cleanedList := make([]vshnv1.VSHNNextcloud, 0)
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
