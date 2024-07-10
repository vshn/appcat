package redis

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type vshnRedisProvider interface {
	ListVSHNRedis(ctx context.Context, namespace string) (*vshnv1.VSHNRedisList, error)
}

type concreteRedisProvider struct {
	client client.Client
}

func (c *concreteRedisProvider) ListVSHNRedis(ctx context.Context, namespace string) (*vshnv1.VSHNRedisList, error) {

	instances := &vshnv1.VSHNRedisList{}

	err := c.client.List(ctx, instances, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, err
	}

	cleanedList := make([]vshnv1.VSHNRedis, 0)
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
