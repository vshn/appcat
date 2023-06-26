package redis

import (
	"context"

	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
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

	return instances, nil
}
