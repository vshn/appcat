package tcproute

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"github.com/vshn/appcat/v4/pkg/controller/webhooks/tcpgateway"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	defaultGatewayNamespaceConfigKey = "tcpGatewayNamespace"
	defaultGatewaysConfigKey         = "tcpGateways"
	gatewayEnabledConfigKey          = "tcpGatewayEnabled"
)

// AddTCPRoute creates the Gateway API resources needed for TCP routing:
// an XListenerSet, a TCPRoute, and a NetworkPolicy.
func AddTCPRoute(svc *runtime.ServiceRuntime, cfg TCPRouteConfig) (*xfnproto.Result, ObservedState) {
	if !svc.GetBoolFromCompositionConfig(gatewayEnabledConfigKey) {
		return runtime.NewWarningResult("gateway not enabled in composition input"), ObservedState{}
	}

	cfg.applyDefaults()

	gatewayNamespace := svc.Config.Data[cfg.GatewayNamespaceConfigKey]
	tcpGatewayName := defaultGatewayName(svc, cfg.GatewaysConfigKey)

	if gatewayNamespace == "" || tcpGatewayName == "" {
		return runtime.NewWarningResult(fmt.Sprintf("TCPRoute requested but %s or %s is not configured", cfg.GatewayNamespaceConfigKey, cfg.GatewaysConfigKey)), ObservedState{}
	}

	svc.Log.Info("Configuring TCPRoute", "resource", cfg.ResourceName)

	observed := observeXListenerSet(svc, cfg.ResourceName+"-xls")

	effectiveGatewayName := tcpGatewayName
	effectiveGatewayNamespace := gatewayNamespace

	if observed.GatewayName != "" {
		effectiveGatewayName = observed.GatewayName
		effectiveGatewayNamespace = observed.GatewayNamespace
	}

	err := createXListenerSet(svc, cfg, effectiveGatewayNamespace, effectiveGatewayName, observed.Port)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create XListenerSet: %s", err)), observed
	}

	err = createTCPRoute(svc, cfg)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create TCPRoute: %s", err)), observed
	}

	err = createGatewayNetworkPolicy(svc, cfg, effectiveGatewayNamespace)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create gateway NetworkPolicy: %s", err)), observed
	}

	observed.Domain = lookupDomain(svc, cfg.GatewaysConfigKey, effectiveGatewayName)

	return nil, observed
}

func createXListenerSet(svc *runtime.ServiceRuntime, cfg TCPRouteConfig, gatewayNamespace, gatewayName string, port int32) error {
	xls := &unstructured.Unstructured{
		Object: map[string]any{},
	}
	xls.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
	xls.SetKind("XListenerSet")
	xls.SetName(cfg.ResourceName)
	xls.SetNamespace(cfg.InstanceNamespace)
	xls.SetLabels(map[string]string{
		runtime.TCPGatewayLabel:     "true",
		tcpgateway.AllowedLabelName: getAllowedGateways(svc, cfg.GatewaysConfigKey),
	})

	err := unstructured.SetNestedMap(xls.Object, map[string]any{
		"group":     "gateway.networking.k8s.io",
		"kind":      "Gateway",
		"namespace": gatewayNamespace,
		"name":      gatewayName,
	}, "spec", "parentRef")
	if err != nil {
		return fmt.Errorf("setting parentRef: %w", err)
	}

	listener := map[string]any{
		"name":     cfg.ListenerName,
		"port":     int64(port),
		"protocol": "TCP",
	}

	err = unstructured.SetNestedField(listener, "Same", "allowedRoutes", "namespaces", "from")
	if err != nil {
		return fmt.Errorf("setting allowedRoutes.namespaces.from: %w", err)
	}

	err = unstructured.SetNestedSlice(listener, []any{
		map[string]any{
			"group": "gateway.networking.k8s.io",
			"kind":  "TCPRoute",
		},
	}, "allowedRoutes", "kinds")
	if err != nil {
		return fmt.Errorf("setting allowedRoutes.kinds: %w", err)
	}

	err = unstructured.SetNestedSlice(xls.Object, []any{listener}, "spec", "listeners")
	if err != nil {
		return fmt.Errorf("setting listeners: %w", err)
	}

	return svc.SetDesiredKubeObject(xls, cfg.ResourceName+"-xls", runtime.KubeOptionAllowDeletion)
}

func createTCPRoute(svc *runtime.ServiceRuntime, cfg TCPRouteConfig) error {
	tcpRoute := &unstructured.Unstructured{
		Object: map[string]any{},
	}
	tcpRoute.SetAPIVersion("gateway.networking.k8s.io/v1alpha2")
	tcpRoute.SetKind("TCPRoute")
	tcpRoute.SetName(cfg.ResourceName)
	tcpRoute.SetNamespace(cfg.InstanceNamespace)

	err := unstructured.SetNestedSlice(tcpRoute.Object, []any{
		map[string]any{
			"group":       "gateway.networking.x-k8s.io",
			"kind":        "XListenerSet",
			"name":        cfg.ResourceName,
			"sectionName": cfg.ListenerName,
		},
	}, "spec", "parentRefs")
	if err != nil {
		return fmt.Errorf("setting parentRefs: %w", err)
	}

	err = unstructured.SetNestedSlice(tcpRoute.Object, []any{
		map[string]any{
			"backendRefs": []any{
				map[string]any{
					"group":     "",
					"kind":      "Service",
					"name":      cfg.BackendServiceName,
					"namespace": cfg.InstanceNamespace,
					"port":      int64(cfg.BackendServicePort),
				},
			},
		},
	}, "spec", "rules")
	if err != nil {
		return fmt.Errorf("setting rules: %w", err)
	}

	return svc.SetDesiredKubeObject(tcpRoute, cfg.ResourceName+"-tcproute", runtime.KubeOptionAllowDeletion)
}

func createGatewayNetworkPolicy(svc *runtime.ServiceRuntime, cfg TCPRouteConfig, gatewayNamespace string) error {
	protocol := corev1.ProtocolTCP
	port := intstr.FromInt(int(cfg.PodListenPort))

	netPol := &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.ResourceName,
			Namespace: cfg.InstanceNamespace,
		},
		Spec: netv1.NetworkPolicySpec{
			PolicyTypes: []netv1.PolicyType{"Ingress"},
			PodSelector: metav1.LabelSelector{
				MatchLabels: cfg.PodSelectorLabels,
			},
			Ingress: []netv1.NetworkPolicyIngressRule{
				{
					From: []netv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": gatewayNamespace,
								},
							},
						},
					},
					Ports: []netv1.NetworkPolicyPort{
						{
							Protocol: &protocol,
							Port:     &port,
						},
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(netPol, cfg.ResourceName+"-gw-netpol", runtime.KubeOptionAllowDeletion)
}

func observeXListenerSet(svc *runtime.ServiceRuntime, name string) ObservedState {
	observed := &unstructured.Unstructured{
		Object: map[string]any{},
	}
	observed.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
	observed.SetKind("XListenerSet")

	err := svc.GetObservedKubeObject(observed, name)
	if err != nil {
		svc.Log.Info("XListenerSet not yet observed, skipping readback")
		return ObservedState{}
	}

	state := ObservedState{}

	gwName, _, _ := unstructured.NestedString(observed.Object, "spec", "parentRef", "name")
	gwNs, _, _ := unstructured.NestedString(observed.Object, "spec", "parentRef", "namespace")
	state.GatewayName = gwName
	state.GatewayNamespace = gwNs

	listeners, found, err := unstructured.NestedSlice(observed.Object, "spec", "listeners")
	if err != nil || !found || len(listeners) == 0 {
		svc.Log.Info("No listeners found in observed XListenerSet")
		return state
	}

	listenerMap, ok := listeners[0].(map[string]any)
	if !ok {
		return state
	}

	state.Port = utils.ToInt32(listenerMap["port"])

	if state.Port == 0 {
		return state
	}

	svc.Log.Info("Observed allocated port", "port", state.Port, "gateway", state.GatewayName)

	return state
}

// lookupDomain resolves the domain for a given gateway name from the
// gateways config value.
func lookupDomain(svc *runtime.ServiceRuntime, configKey, gatewayName string) string {
	raw := svc.Config.Data[configKey]
	if raw == "" {
		return ""
	}

	mapping := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
		svc.Log.Error(err, "failed to parse gateways config", "key", configKey)
		svc.AddResult(runtime.NewFatalResult(fmt.Errorf("failed to parse gateways: %w", err)))
		return ""
	}

	return mapping[gatewayName]
}

func defaultGatewayName(svc *runtime.ServiceRuntime, configKey string) string {
	mapping := getRawGateways(svc, configKey)

	names := make([]string, 0, len(mapping))
	for name := range mapping {
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func getRawGateways(svc *runtime.ServiceRuntime, configKey string) map[string]string {
	raw, ok := svc.Config.Data[configKey]
	if !ok || raw == "" {
		return map[string]string{}
	}

	mapping := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
		svc.Log.Error(err, "failed to parse gateways config", "key", configKey)
		return mapping
	}

	return mapping
}

func getAllowedGateways(svc *runtime.ServiceRuntime, configKey string) string {
	mapping := getRawGateways(svc, configKey)

	names := make([]string, 0, len(mapping))
	for name := range mapping {
		names = append(names, name)
	}
	sort.Strings(names)

	return strings.Join(names, ",")
}
