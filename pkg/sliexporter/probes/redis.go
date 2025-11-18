package probes

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

var (
	redisMasterGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "appcat_probes_redis_ha_master_up",
			Help: "Redis HA master status (1 if master, 0 if not)",
		},
		[]string{"service", "claim_namespace", "instance_namespace", "name", "organization", "ha", "sla", "maintenance"},
	)

	redisQuorumGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "appcat_probes_redis_ha_quorum_ok",
			Help: "Redis HA quorum status (1 if quorum is healthy, 0 if not)",
		},
		[]string{"service", "claim_namespace", "instance_namespace", "name", "organization", "ha", "sla", "maintenance"},
	)
)

// VSHNRedis implements Prober for Redis.
type VSHNRedis struct {
	redisClient       *redis.Client
	sentinelClient    *redis.Client
	Service           string
	Name              string
	ClaimNamespace    string
	InstanceNamespace string
	HighAvailable     bool
	Organization      string
	ServiceLevel      string
	CompositionName   string
}

func NewRedis(service, name, claimNamespace, instanceNamespace, organization, sla, compositionName string, ha bool, opts redis.Options) (*VSHNRedis, error) {
	client := redis.NewClient(&opts)
	r := &VSHNRedis{
		redisClient:       client,
		Service:           service,
		Name:              name,
		ClaimNamespace:    claimNamespace,
		InstanceNamespace: instanceNamespace,
		HighAvailable:     ha,
		Organization:      organization,
		ServiceLevel:      sla,
		CompositionName:   compositionName,
	}

	if ha {
		parts := strings.Split(opts.Addr, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid redis address format: %s", opts.Addr)
		}

		hostParts := strings.SplitN(parts[0], ".", 2)
		if len(hostParts) != 2 {
			return nil, fmt.Errorf("invalid redis host format: %s", parts[0])
		}

		headlessHost := "redis-headless." + hostParts[1]

		sentinelOpts := &redis.Options{
			Addr:      fmt.Sprintf("%s:26379", headlessHost),
			Username:  opts.Username,
			Password:  opts.Password,
			TLSConfig: opts.TLSConfig,
		}
		r.sentinelClient = redis.NewClient(sentinelOpts)
	}

	return r, nil
}

func (redis *VSHNRedis) Close() error {
	var errors []error

	if redis.redisClient != nil {
		if err := redis.redisClient.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if redis.sentinelClient != nil {
		if err := redis.sentinelClient.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing clients: %v", errors)
	}
	return nil
}

func (redis *VSHNRedis) GetInfo() ProbeInfo {
	return ProbeInfo{
		Service:           redis.Service,
		Name:              redis.Name,
		ClaimNamespace:    redis.ClaimNamespace,
		InstanceNamespace: redis.InstanceNamespace,
		HighAvailable:     redis.HighAvailable,
		Organization:      redis.Organization,
		ServiceLevel:      redis.ServiceLevel,
		CompositionName:   redis.CompositionName,
	}
}

// GetRedisCollectors returns all redis-specific prometheus collectors to register.
func GetRedisCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		redisMasterGauge,
		redisQuorumGauge,
	}
}

func (redis *VSHNRedis) Probe(ctx context.Context) error {
	labels := redis.labels(maintenanceFromContext(ctx))

	if _, err := redis.redisClient.Ping(ctx).Result(); err != nil {
		return err
	}

	if !redis.HighAvailable {
		return nil
	}

	if err := redis.validateMasterRole(ctx); err != nil {
		redisMasterGauge.With(labels).Set(0)
	} else {
		redisMasterGauge.With(labels).Set(1)
	}

	if err := redis.validateQuorum(ctx); err != nil {
		redisQuorumGauge.With(labels).Set(0)
	} else {
		redisQuorumGauge.With(labels).Set(1)
	}

	return nil
}

func (redis *VSHNRedis) validateMasterRole(ctx context.Context) error {
	res, err := redis.redisClient.Do(ctx, "ROLE").Slice()
	if err != nil {
		return fmt.Errorf("ROLE command failed: %w", err)
	}

	if len(res) == 0 {
		return fmt.Errorf("ROLE: empty response")
	}

	role := res[0].(string)
	if role != "master" {
		return fmt.Errorf("connected role=%s, expected master for HA service", role)
	}

	return nil
}

func (redis *VSHNRedis) validateQuorum(ctx context.Context) error {
	if redis.sentinelClient == nil {
		return fmt.Errorf("sentinel client not configured for HA setup")
	}

	if _, err := redis.sentinelClient.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("sentinel ping failed: %w", err)
	}

	res, err := redis.sentinelClient.Do(ctx, "SENTINEL", "CKQUORUM", "mymaster").Result()
	if err != nil {
		return fmt.Errorf("SENTINEL CKQUORUM failed: %w", err)
	}

	if str, ok := res.(string); !ok || !strings.HasPrefix(str, "OK") {
		return fmt.Errorf("quorum check failed: %v", res)
	}

	return nil
}

func (redis *VSHNRedis) labels(maintenance bool) prometheus.Labels {
	return prometheus.Labels{
		"service":            redis.Service,
		"claim_namespace":    redis.ClaimNamespace,
		"instance_namespace": redis.InstanceNamespace,
		"name":               redis.Name,
		"organization":       redis.Organization,
		"ha":                 strconv.FormatBool(redis.HighAvailable),
		"sla":                redis.ServiceLevel,
		"maintenance":        strconv.FormatBool(maintenance),
	}
}
