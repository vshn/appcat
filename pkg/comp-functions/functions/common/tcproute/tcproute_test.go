package tcproute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func testConfig() TCPRouteConfig {
	return TCPRouteConfig{
		ResourceName:       "my-service-tcp",
		ListenerName:       "tcp",
		BackendServiceName: "my-backend-svc",
		BackendServicePort: 3306,
		PodListenPort:      3306,
		PodSelectorLabels: map[string]string{
			"app.kubernetes.io/name":     "myapp",
			"app.kubernetes.io/instance": "my-service",
		},
		InstanceNamespace: "vshn-myapp-test",
	}
}

func TestAddTCPRoute(t *testing.T) {
	t.Run("AllResourcesCreated", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "tcproute/01_basic.yaml")
		cfg := testConfig()

		result, state := AddTCPRoute(svc, cfg)
		assert.Nil(t, result)
		assert.Equal(t, int32(0), state.Port, "port should be 0 when no observed XListenerSet")

		// Verify XListenerSet
		xls := &unstructured.Unstructured{}
		xls.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
		xls.SetKind("XListenerSet")
		require.NoError(t, svc.GetDesiredKubeObject(xls, cfg.ResourceName+"-xls"))
		assert.Equal(t, cfg.ResourceName, xls.GetName())
		assert.Equal(t, cfg.InstanceNamespace, xls.GetNamespace())

		parentName, _, _ := unstructured.NestedString(xls.Object, "spec", "parentRef", "name")
		parentNs, _, _ := unstructured.NestedString(xls.Object, "spec", "parentRef", "namespace")
		assert.Equal(t, "tcp-gateway", parentName)
		assert.Equal(t, "gateway-system", parentNs)

		// Verify allowed-gateways annotation
		ann := xls.GetAnnotations()
		assert.Equal(t, "tcp-gateway", ann["appcat.vshn.io/allowed-gateways"])

		listeners, found, _ := unstructured.NestedSlice(xls.Object, "spec", "listeners")
		require.True(t, found)
		require.Len(t, listeners, 1)
		l0 := listeners[0].(map[string]any)
		assert.Equal(t, "tcp", l0["name"])
		assert.Equal(t, "TCP", l0["protocol"])
		assert.Equal(t, int64(0), l0["port"])

		// Verify allowedRoutes scoped to same namespace (XLS and TCPRoute co-located)
		fromMode, _, _ := unstructured.NestedString(l0, "allowedRoutes", "namespaces", "from")
		assert.Equal(t, "Same", fromMode)

		// Verify TCPRoute
		tcpRoute := &unstructured.Unstructured{}
		tcpRoute.SetAPIVersion("gateway.networking.k8s.io/v1alpha2")
		tcpRoute.SetKind("TCPRoute")
		require.NoError(t, svc.GetDesiredKubeObject(tcpRoute, cfg.ResourceName+"-tcproute"))
		assert.Equal(t, cfg.ResourceName, tcpRoute.GetName())
		assert.Equal(t, cfg.InstanceNamespace, tcpRoute.GetNamespace())

		parentRefs, _, _ := unstructured.NestedSlice(tcpRoute.Object, "spec", "parentRefs")
		require.Len(t, parentRefs, 1)
		pRef := parentRefs[0].(map[string]any)
		assert.Equal(t, "XListenerSet", pRef["kind"])
		assert.Equal(t, cfg.ResourceName, pRef["name"])
		assert.Equal(t, "tcp", pRef["sectionName"])
		assert.Nil(t, pRef["namespace"], "parentRef must have no namespace (same-ns)")

		rules, _, _ := unstructured.NestedSlice(tcpRoute.Object, "spec", "rules")
		require.Len(t, rules, 1)
		backendRefs := rules[0].(map[string]any)["backendRefs"].([]any)
		require.Len(t, backendRefs, 1)
		backend := backendRefs[0].(map[string]any)
		assert.Equal(t, "Service", backend["kind"])
		assert.Equal(t, cfg.BackendServiceName, backend["name"])
		assert.Equal(t, cfg.InstanceNamespace, backend["namespace"])
		assert.Equal(t, int64(cfg.BackendServicePort), backend["port"])

		// Verify NetworkPolicy
		netPol := &netv1.NetworkPolicy{}
		require.NoError(t, svc.GetDesiredKubeObject(netPol, cfg.ResourceName+"-gw-netpol"))
		assert.Equal(t, cfg.ResourceName, netPol.Name)
		assert.Equal(t, cfg.InstanceNamespace, netPol.Namespace)
		assert.Equal(t, "myapp", netPol.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])
		assert.Equal(t, "my-service", netPol.Spec.PodSelector.MatchLabels["app.kubernetes.io/instance"])
		require.Len(t, netPol.Spec.Ingress, 1)
		require.Len(t, netPol.Spec.Ingress[0].From, 1)
		assert.Equal(t, "gateway-system", netPol.Spec.Ingress[0].From[0].NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"])
		require.Len(t, netPol.Spec.Ingress[0].Ports, 1)
		assert.Equal(t, int32(cfg.PodListenPort), netPol.Spec.Ingress[0].Ports[0].Port.IntVal)
	})

	t.Run("GatewayConfigMissing_WarningResult", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "tcproute/01_basic.yaml")
		cfg := testConfig()

		delete(svc.Config.Data, "tcpGatewayNamespace")
		delete(svc.Config.Data, "tcpGateways")

		result, _ := AddTCPRoute(svc, cfg)
		require.NotNil(t, result)
		assert.Contains(t, result.Message, "is not configured")
	})

	t.Run("AllocatedPortPreserved", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "tcproute/02_with_port.yaml")
		cfg := testConfig()

		result, state := AddTCPRoute(svc, cfg)
		assert.Nil(t, result)

		// Verify port preserved from observed state
		assert.Equal(t, int32(10005), state.Port)
		assert.Equal(t, "tcp.example.com", state.Domain)

		// Verify XListenerSet has the allocated port
		xls := &unstructured.Unstructured{}
		xls.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
		xls.SetKind("XListenerSet")
		require.NoError(t, svc.GetDesiredKubeObject(xls, cfg.ResourceName+"-xls"))
		listeners, _, _ := unstructured.NestedSlice(xls.Object, "spec", "listeners")
		require.Len(t, listeners, 1)
		l0 := listeners[0].(map[string]any)
		assert.Equal(t, int64(10005), l0["port"])
	})

	t.Run("AllocatedGatewayPreserved", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "tcproute/03_sharded.yaml")
		cfg := testConfig()

		result, state := AddTCPRoute(svc, cfg)
		assert.Nil(t, result)

		// Verify gateway preserved from observed state
		assert.Equal(t, int32(10005), state.Port)
		assert.Equal(t, "tcp2.example.com", state.Domain)

		// Verify XListenerSet uses observed gateway
		xls := &unstructured.Unstructured{}
		xls.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
		xls.SetKind("XListenerSet")
		require.NoError(t, svc.GetDesiredKubeObject(xls, cfg.ResourceName+"-xls"))

		parentName, _, _ := unstructured.NestedString(xls.Object, "spec", "parentRef", "name")
		parentNs, _, _ := unstructured.NestedString(xls.Object, "spec", "parentRef", "namespace")
		assert.Equal(t, "tcp-gateway-2", parentName)
		assert.Equal(t, "gateway-system", parentNs)

		// Verify allowed-gateways annotation contains both gateways (sorted)
		ann := xls.GetAnnotations()
		assert.Equal(t, "tcp-gateway,tcp-gateway-2", ann["appcat.vshn.io/allowed-gateways"])
	})

	t.Run("NoResources_WhenNotCalled", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "tcproute/01_basic.yaml")
		cfg := testConfig()

		// Verify no resources exist before calling AddTCPRoute
		xls := &unstructured.Unstructured{}
		xls.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
		xls.SetKind("XListenerSet")
		assert.ErrorIs(t, svc.GetDesiredKubeObject(xls, cfg.ResourceName+"-xls"), runtime.ErrNotFound)
	})
}
