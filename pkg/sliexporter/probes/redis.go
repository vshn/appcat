package probes

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type VSHNRedis struct {
	redisClient   redis.Client
	Service       string
	Name          string
	Namespace     string
	HighAvailable bool
	Organization  string
	ServiceLevel  string
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
		Service:       redis.Service,
		Name:          redis.Name,
		Namespace:     redis.Namespace,
		HighAvailable: false,
		Organization:  redis.Organization,
		ServiceLevel:  redis.ServiceLevel,
	}
}

func (redis VSHNRedis) Probe(ctx context.Context) error {

	_, err := redis.redisClient.Ping(ctx).Result()
	if err != nil {
		return err
	}
	return nil
}

func NewRedis(service, name, namespace, organization, sla string, ha bool, opts redis.Options) (*VSHNRedis, error) {

	client := redis.NewClient(&opts)

	return &VSHNRedis{
		redisClient:   *client,
		Service:       service,
		Name:          name,
		Namespace:     namespace,
		HighAvailable: ha,
		Organization:  organization,
		ServiceLevel:  sla,
	}, nil
}
