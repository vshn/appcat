package vshnforgejo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	sshListenerName = "ssh"
	sshPort         = 22
)

// ConfigureSSHAccess creates the Gateway API resources needed for TCP routing
// when SSH access is enabled on a Forgejo instance.
func ConfigureSSHAccess(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	if !comp.Spec.Parameters.Service.SSH.Enabled {
		return nil
	}

	gatewayNamespace := svc.Config.Data["sshGatewayNamespace"]
	tcpGatewayName := defaultGatewayName(svc)

	if gatewayNamespace == "" || tcpGatewayName == "" {
		return runtime.NewWarningResult("SSH is enabled but sshGatewayNamespace or sshGateways is not configured")
	}

	svc.Log.Info("Configuring SSH access for Forgejo instance")

	resourceBaseName := comp.GetName() + "-ssh"

	// Observe the XListenerSet to get the allocated port and gateway (if it exists already).
	// We need this before creating the desired XListenerSet so we preserve the
	// webhook-assigned port and gateway instead of resetting them.
	observed := observeXListenerSet(svc, resourceBaseName)

	// Use observed gateway if available (webhook may have reassigned it via sharding),
	// otherwise fall back to config.
	effectiveGatewayName := tcpGatewayName
	effectiveGatewayNamespace := gatewayNamespace
	if observed.gatewayName != "" {
		effectiveGatewayName = observed.gatewayName
		effectiveGatewayNamespace = observed.gatewayNamespace
	}

	// Create XListenerSet — use allocated port if known, otherwise 0 (webhook will assign on CREATE)
	err = createXListenerSet(svc, resourceBaseName, effectiveGatewayNamespace, effectiveGatewayName, observed.port)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create XListenerSet: %s", err))
	}

	// Create TCPRoute pointing to the Forgejo SSH service
	err = createTCPRoute(svc, comp, resourceBaseName, gatewayNamespace)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create TCPRoute: %s", err))
	}

	// Create ReferenceGrant allowing TCPRoute cross-namespace reference
	err = createReferenceGrant(svc, comp, resourceBaseName, gatewayNamespace)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create ReferenceGrant: %s", err))
	}

	// Allow the gateway namespace to reach the Forgejo SSH service
	err = createGatewayNetworkPolicy(svc, comp, resourceBaseName, gatewayNamespace)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create gateway NetworkPolicy: %s", err))
	}

	// Resolve the SSH domain for this gateway from the domain mapping
	sshDomain := lookupSSHDomain(svc, effectiveGatewayName)

	// Publish port/domain in status and connection details once we have an allocated port
	if observed.port > 0 {
		comp.Status.SSHPort = observed.port
		if err := svc.SetDesiredCompositeStatus(comp); err != nil {
			svc.Log.Error(err, "cannot update SSHPort in status")
		}
		if sshDomain != "" {
			svc.SetConnectionDetail("FORGEJO_SSH_HOST", []byte(sshDomain))
		}
		svc.SetConnectionDetail("FORGEJO_SSH_PORT", []byte(strconv.FormatInt(int64(observed.port), 10)))
	}

	// Enable SSH in the Helm release values and set the clone URL domain/port
	err = enableSSHInRelease(svc, comp, sshDomain, observed.port)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot enable SSH in release: %s", err))
	}

	return nil
}

func createXListenerSet(svc *runtime.ServiceRuntime, name, namespace, gatewayName string, port int32) error {
	xls := &unstructured.Unstructured{
		Object: map[string]any{},
	}
	xls.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
	xls.SetKind("XListenerSet")
	xls.SetName(name)
	xls.SetNamespace(namespace)

	xls.Object["spec"] = map[string]any{
		"parentRef": map[string]any{
			"group":     "gateway.networking.k8s.io",
			"kind":      "Gateway",
			"namespace": namespace,
			"name":      gatewayName,
		},
		"listeners": []any{
			map[string]any{
				"name":     sshListenerName,
				"port":     int64(port),
				"protocol": "TCP",
				"allowedRoutes": map[string]any{
					"kinds": []any{
						map[string]any{
							"group": "gateway.networking.k8s.io",
							"kind":  "TCPRoute",
						},
					},
					"namespaces": map[string]any{
						"from": "All",
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(xls, name)
}

func createTCPRoute(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo, name, gatewayNamespace string) error {
	instanceNs := comp.GetInstanceNamespace()

	// The Forgejo Helm chart creates a service named <fullname>-ssh for the SSH port.
	sshServiceName := helmFullname(comp) + "-ssh"

	tcpRoute := &unstructured.Unstructured{
		Object: map[string]any{},
	}
	tcpRoute.SetAPIVersion("gateway.networking.k8s.io/v1alpha2")
	tcpRoute.SetKind("TCPRoute")
	tcpRoute.SetName(name)
	tcpRoute.SetNamespace(gatewayNamespace)

	tcpRoute.Object["spec"] = map[string]any{
		"parentRefs": []any{
			map[string]any{
				"group":       "gateway.networking.x-k8s.io",
				"kind":        "XListenerSet",
				"namespace":   gatewayNamespace,
				"name":        name,
				"sectionName": sshListenerName,
			},
		},
		"rules": []any{
			map[string]any{
				"backendRefs": []any{
					map[string]any{
						"group":     "",
						"kind":      "Service",
						"name":      sshServiceName,
						"namespace": instanceNs,
						"port":      int64(sshPort),
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(tcpRoute, name+"-tcproute")
}

func createReferenceGrant(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo, name, gatewayNamespace string) error {
	instanceNs := comp.GetInstanceNamespace()
	sshServiceName := helmFullname(comp) + "-ssh"

	refGrant := &unstructured.Unstructured{
		Object: map[string]any{},
	}
	refGrant.SetAPIVersion("gateway.networking.k8s.io/v1beta1")
	refGrant.SetKind("ReferenceGrant")
	refGrant.SetName(name)
	refGrant.SetNamespace(instanceNs)

	refGrant.Object["spec"] = map[string]any{
		"from": []any{
			map[string]any{
				"group":     "gateway.networking.k8s.io",
				"kind":      "TCPRoute",
				"namespace": gatewayNamespace,
			},
		},
		"to": []any{
			map[string]any{
				"group": "",
				"kind":  "Service",
				"name":  sshServiceName,
			},
		},
	}

	return svc.SetDesiredKubeObject(refGrant, name+"-refgrant")
}

func createGatewayNetworkPolicy(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo, name, gatewayNamespace string) error {
	instanceNs := comp.GetInstanceNamespace()

	netPol := &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instanceNs,
		},
		Spec: netv1.NetworkPolicySpec{
			PolicyTypes: []netv1.PolicyType{"Ingress"},
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":     comp.GetServiceName(),
					"app.kubernetes.io/instance": helmFullname(comp),
				},
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
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(netPol, name+"-netpol")
}

// enableSSHInRelease modifies the Helm release values to enable SSH in Forgejo.
func enableSSHInRelease(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo, sshDomain string, allocatedPort int32) error {
	release := &xhelmv1.Release{}
	err := svc.GetDesiredComposedResourceByName(release, comp.GetName())
	if err != nil {
		return fmt.Errorf("getting release: %w", err)
	}

	values, err := common.GetReleaseValues(release)
	if err != nil {
		return fmt.Errorf("getting release values: %w", err)
	}

	common.SetNestedObjectValue(values, []string{"gitea", "config", "server", "DISABLE_SSH"}, false)
	common.SetNestedObjectValue(values, []string{"gitea", "config", "server", "START_SSH_SERVER"}, true)

	if sshDomain != "" {
		common.SetNestedObjectValue(values, []string{"gitea", "config", "server", "SSH_DOMAIN"}, sshDomain)
	}
	if allocatedPort > 0 {
		common.SetNestedObjectValue(values, []string{"gitea", "config", "server", "SSH_PORT"}, int(allocatedPort))
	}

	byteValues, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("marshaling values: %w", err)
	}
	release.Spec.ForProvider.Values.Raw = byteValues

	return svc.SetDesiredComposedResourceWithName(release, comp.GetName())
}

// lookupSSHDomain resolves the SSH domain for a given gateway name from the
// sshGateways config value.
func lookupSSHDomain(svc *runtime.ServiceRuntime, gatewayName string) string {
	raw := svc.Config.Data["sshGateways"]
	if raw == "" {
		return ""
	}

	mapping := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
		svc.Log.Error(err, "failed to parse sshGateways")
		return ""
	}

	return mapping[gatewayName]
}

// observedXListenerSetState holds the observed state of an XListenerSet,
// including the webhook-assigned port and gateway.
type observedXListenerSetState struct {
	port             int32
	gatewayName      string
	gatewayNamespace string
}

func observeXListenerSet(svc *runtime.ServiceRuntime, name string) observedXListenerSetState {
	observed := &unstructured.Unstructured{
		Object: map[string]any{},
	}
	observed.SetAPIVersion("gateway.networking.x-k8s.io/v1alpha1")
	observed.SetKind("XListenerSet")

	err := svc.GetObservedKubeObject(observed, name)
	if err != nil {
		svc.Log.Info("XListenerSet not yet observed, skipping readback")
		return observedXListenerSetState{}
	}

	state := observedXListenerSetState{}

	gwName, _, _ := unstructured.NestedString(observed.Object, "spec", "parentRef", "name")
	gwNs, _, _ := unstructured.NestedString(observed.Object, "spec", "parentRef", "namespace")
	state.gatewayName = gwName
	state.gatewayNamespace = gwNs

	listeners, found, err := unstructured.NestedSlice(observed.Object, "spec", "listeners")
	if err != nil || !found || len(listeners) == 0 {
		svc.Log.Info("No listeners found in observed XListenerSet")
		return state
	}

	listenerMap, ok := listeners[0].(map[string]any)
	if !ok {
		return state
	}

	state.port = toInt32(listenerMap["port"])

	if state.port == 0 {
		return state
	}

	svc.Log.Info("Observed allocated SSH port", "port", state.port, "gateway", state.gatewayName)

	return state
}

func defaultGatewayName(svc *runtime.ServiceRuntime) string {
	raw := svc.Config.Data["sshGateways"]
	if raw == "" {
		return ""
	}

	mapping := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
		svc.Log.Error(err, "failed to parse sshGateways")
		return ""
	}

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

func toInt32(v any) int32 {
	switch p := v.(type) {
	case int64:
		return int32(p)
	case float64:
		return int32(p)
	default:
		return 0
	}
}
