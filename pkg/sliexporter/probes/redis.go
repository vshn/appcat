package probes

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type VSHNRedis struct {
	redisClient  redis.Client
	Service      string
	Instance     string
	Namespace    string
	Organization string
}

func (redis VSHNRedis) Close() error {
	// Redis requires context here
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if redis.redisClient.ClientID(ctx) != nil {
		redis.redisClient.Close()
	}
	return nil
}

func (redis VSHNRedis) GetInfo() ProbeInfo {
	return ProbeInfo{
		Service:      redis.Service,
		Name:         redis.Instance,
		Namespace:    redis.Namespace,
		Organization: redis.Organization,
	}
}

func (redis VSHNRedis) Probe(ctx context.Context) error {

	_, err := redis.redisClient.Ping(ctx).Result()
	if err != nil {
		return err
	}
	return nil
}

func NewRedis(service, name, namespace, organization string, opts redis.Options) (*VSHNRedis, error) {

	client := redis.NewClient(&opts)

	return &VSHNRedis{
		redisClient:  *client,
		Service:      service,
		Instance:     name,
		Namespace:    namespace,
		Organization: organization,
	}, nil
}
