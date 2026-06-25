package vshnmariadb

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
	mariadbListenerName = "mariadb"
	// mainServiceName is the stable client service created by createMainService.
	// It exposes port 3306, routing to the mariadb pod (single) or proxysql (HA).
	mainServiceName       = "mariadb"
	mariadbServicePort    = 3306
	mariadbPodListenPort  = 3306
	proxysqlPodListenPort = 6033
)

// ConfigureTCPGateway creates Gateway API resources for external TCP access to
// the MariaDB instance when ServiceType is set to "TCPGateway".
func ConfigureTCPGateway(ctx context.Context, comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	if comp.Spec.Parameters.Network.ServiceType != tcproute.ServiceTypeTCPGateway {
		return nil
	}

	if !externalAccessEnabled(svc) {
		return runtime.NewWarningResult("TCPGateway requested but external database connections are not enabled")
	}

	// The client service always listens on 3306. The pods behind it differ by
	// topology: single instance is the mariadb pod, HA is fronted by proxysql.
	podSelector := map[string]string{"app": "mariadb"}
	var podListenPort int32 = mariadbPodListenPort
	if comp.GetInstances() > 1 {
		podSelector = map[string]string{"app": "proxysql"}
		podListenPort = proxysqlPodListenPort
	}

	cfg := tcproute.TCPRouteConfig{
		ResourceName:       comp.GetName(),
		ListenerName:       mariadbListenerName,
		BackendServiceName: mainServiceName,
		BackendServicePort: mariadbServicePort,
		PodListenPort:      podListenPort,
		PodSelectorLabels:  podSelector,
		InstanceNamespace:  comp.GetInstanceNamespace(),
	}

	result, state := tcproute.AddTCPRoute(svc, cfg)
	if result != nil {
		return result
	}

	if state.Port > 0 && state.Domain != "" {
		svc.SetConnectionDetail("MARIADB_GATEWAY_HOST", []byte(state.Domain))
		svc.SetConnectionDetail("MARIADB_GATEWAY_PORT", []byte(strconv.FormatInt(int64(state.Port), 10)))
	}

	return nil
}
