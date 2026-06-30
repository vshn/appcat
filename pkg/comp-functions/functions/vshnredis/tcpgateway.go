package vshnredis

import (
	"context"
	"fmt"
	"strconv"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/tcproute"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

const (
	redisListenerName = "redis"
	// redisServiceName is the sentinel client service, used as backend for single-instance deployments.
	redisServiceName = "redis"
	// redisMasterServiceName is the master-following service, used as backend when instances > 1.
	redisMasterServiceName  = "redis-master"
	redisBackendServicePort = 6379
	redisPodListenPort      = 6379
)

// ConfigureTCPGateway creates Gateway API resources for external TCP access
// to the Redis instance when ServiceType is set to "TCPGateway".
func ConfigureTCPGateway(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	if comp.Spec.Parameters.Network.ServiceType != tcproute.ServiceTypeTCPGateway {
		return nil
	}

	if !common.ExternalAccessEnabled(svc) {
		return runtime.NewWarningResult("TCPGateway requested but external connections are not enabled")
	}

	// Single instance is served by the main redis service.
	// HA exposes the master-following service.
	backendServiceName := redisServiceName
	if comp.GetInstances() > 1 {
		backendServiceName = redisMasterServiceName
	}

	cfg := tcproute.TCPRouteConfig{
		ResourceName:       comp.GetName(),
		ListenerName:       redisListenerName,
		BackendServiceName: backendServiceName,
		BackendServicePort: redisBackendServicePort,
		PodListenPort:      redisPodListenPort,
		PodSelectorLabels: map[string]string{
			"app.kubernetes.io/instance": comp.GetName(),
			"app.kubernetes.io/name":     "redis",
		},
		InstanceNamespace: comp.GetInstanceNamespace(),
	}

	result, state := tcproute.AddTCPRoute(svc, cfg)
	if result != nil {
		return result
	}

	if state.Port > 0 && state.Domain != "" {
		svc.SetConnectionDetail("REDIS_GATEWAY_HOST", []byte(state.Domain))
		svc.SetConnectionDetail("REDIS_GATEWAY_PORT", []byte(strconv.FormatInt(int64(state.Port), 10)))
	}

	return nil
}
