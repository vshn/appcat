package vshnpostgrescnpg

import (
	"context"
	"fmt"
	"strconv"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/tcproute"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

const (
	pgListenerName       = "postgresql"
	pgBackendServiceName = "postgresql-rw"
	pgBackendServicePort = 5432
	pgPodListenPort      = 5432
)

// ConfigureTCPGateway creates Gateway API resources for external TCP access
// to the PostgreSQL instance when ServiceType is set to "TCPGateway".
func ConfigureTCPGateway(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	if comp.Spec.Parameters.Network.ServiceType != tcproute.ServiceTypeTCPGateway {
		return nil
	}

	if !externalAccessEnabled(svc) {
		return runtime.NewWarningResult("TCPGateway requested but external database connections are not enabled")
	}

	cfg := tcproute.TCPRouteConfig{
		ResourceName:       comp.GetName(),
		ListenerName:       pgListenerName,
		BackendServiceName: pgBackendServiceName,
		BackendServicePort: pgBackendServicePort,
		PodListenPort:      pgPodListenPort,
		PodSelectorLabels: map[string]string{
			"cnpg.io/cluster": "postgresql",
		},
		InstanceNamespace: comp.GetInstanceNamespace(),
	}

	result, state := tcproute.AddTCPRoute(svc, cfg)
	if result != nil {
		return result
	}

	if state.Port > 0 && state.Domain != "" {
		svc.SetConnectionDetail("POSTGRESQL_GATEWAY_HOST", []byte(state.Domain))
		svc.SetConnectionDetail("POSTGRESQL_GATEWAY_PORT", []byte(strconv.FormatInt(int64(state.Port), 10)))
	}

	return nil
}
