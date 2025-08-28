package probes

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

var (
	redisMasterGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "appcat_probes_redis_ha_master_up",
			Help: "Redis HA master status (1 if master, 0 if not)",
		},
		[]string{"service", "namespace", "name", "organization", "ha", "sla"},
	)

	redisQuorumGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "appcat_probes_redis_ha_quorum_ok",
			Help: "Redis HA quorum status (1 if quorum is healthy, 0 if not)",
		},
		[]string{"service", "namespace", "name", "organization", "ha", "sla"},
	)
)

// VSHNRedis implements Prober for Redis.
type VSHNRedis struct {
	redisClient   *redis.Client
	Service       string
	Name          string
	Namespace     string
	HighAvailable bool
	Organization  string
	ServiceLevel  string
}

func NewRedis(service, name, namespace, organization, sla string, ha bool, opts redis.Options) (*VSHNRedis, error) {
	client := redis.NewClient(&opts)
	return &VSHNRedis{
		redisClient:   client,
		Service:       service,
		Name:          name,
		Namespace:     namespace,
		HighAvailable: ha,
		Organization:  organization,
		ServiceLevel:  sla,
	}, nil
}

func (redis *VSHNRedis) Close() error {
	if redis.redisClient != nil {
		return redis.redisClient.Close()
	}
	return nil
}

func (redis *VSHNRedis) GetInfo() ProbeInfo {
	return ProbeInfo{
		Service:       redis.Service,
		Name:          redis.Name,
		Namespace:     redis.Namespace,
		HighAvailable: redis.HighAvailable,
		Organization:  redis.Organization,
		ServiceLevel:  redis.ServiceLevel,
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
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	labels := redis.labels()

	if _, err := redis.redisClient.Ping(probeCtx).Result(); err != nil {
		return err
	}

	if !redis.HighAvailable {
		return nil
	}

	if err := redis.validateMasterRole(probeCtx); err != nil {
		redisMasterGauge.With(labels).Set(0)
	} else {
		redisMasterGauge.With(labels).Set(1)
	}

	if err := redis.validateQuorum(probeCtx); err != nil {
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
	role, _ := res[0].(string)
	if role != "master" {
		return fmt.Errorf("connected role=%s, expected master for HA service", role)
	}
	return nil
}

func (redis *VSHNRedis) validateQuorum(ctx context.Context) error {
	const desiredSlaves = 2

	info, err := redis.redisClient.Info(ctx, "replication").Result()
	if err != nil {
		return fmt.Errorf("INFO replication failed: %w", err)
	}

	connectedSlaves := parseIntFromInfo(info, "connected_slaves")

	totalDesired := 1 + desiredSlaves
	required := (totalDesired / 2) + 1
	available := 1 + connectedSlaves
	if available < required {
		return fmt.Errorf("quorum insufficient: available=%d required=%d (connected_slaves=%d, desired_replicas=%d)", available, required, connectedSlaves, desiredSlaves)
	}
	return nil
}

func (redis *VSHNRedis) labels() prometheus.Labels {
	return prometheus.Labels{
		"service":      redis.Service,
		"namespace":    redis.Namespace,
		"name":         redis.Name,
		"organization": redis.Organization,
		"ha":           strconv.FormatBool(redis.HighAvailable),
		"sla":          redis.ServiceLevel,
	}
}

func parseValueFromInfo(info, key string) string {
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if kv := strings.SplitN(line, ":", 2); len(kv) == 2 {
			k := strings.TrimSpace(kv[0])
			if k == key {
				return strings.TrimSpace(kv[1])
			}
		}
	}
	return ""
}

func parseIntFromInfo(info, key string) int {
	if v := parseValueFromInfo(info, key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return 0
}
