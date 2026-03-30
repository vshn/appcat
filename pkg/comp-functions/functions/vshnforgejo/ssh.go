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
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	observed := observeXListenerSet(svc, resourceBaseName)

	effectiveGatewayName := tcpGatewayName
	effectiveGatewayNamespace := gatewayNamespace

	if observed.gatewayName != "" {
		effectiveGatewayName = observed.gatewayName
		effectiveGatewayNamespace = observed.gatewayNamespace
	}

	err = createXListenerSet(svc, resourceBaseName, effectiveGatewayNamespace, effectiveGatewayName, observed.port)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create XListenerSet: %s", err))
	}

	err = createTCPRoute(svc, comp, resourceBaseName, effectiveGatewayNamespace)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create TCPRoute: %s", err))
	}

	err = createReferenceGrant(svc, comp, resourceBaseName, effectiveGatewayNamespace)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create ReferenceGrant: %s", err))
	}

	err = createGatewayNetworkPolicy(svc, comp, resourceBaseName, effectiveGatewayNamespace)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create gateway NetworkPolicy: %s", err))
	}

	sshDomain := lookupSSHDomain(svc, effectiveGatewayName)

	if observed.port > 0 {
		comp.Status.SSHPort = observed.port
		if err := svc.SetDesiredCompositeStatus(comp); err != nil {
			svc.Log.Error(err, "cannot update SSHPort in status")
		}
		if sshDomain != "" {
			svc.SetConnectionDetail("FORGEJO_SSH_HOST", []byte(sshDomain))
		}
		svc.SetConnectionDetail("FORGEJO_SSH_PORT", []byte(strconv.FormatInt(int64(observed.port), 10)))

		if err := enableSSHInRelease(svc, comp, sshDomain, observed.port); err != nil {
			return runtime.NewWarningResult(fmt.Sprintf("cannot enable SSH in release: %s", err))
		}
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

	err := unstructured.SetNestedMap(xls.Object, map[string]any{
		"group":     "gateway.networking.k8s.io",
		"kind":      "Gateway",
		"namespace": namespace,
		"name":      gatewayName,
	}, "spec", "parentRef")

	if err != nil {
		return fmt.Errorf("setting parentRef: %w", err)
	}

	listener := map[string]any{
		"name":     sshListenerName,
		"port":     int64(port),
		"protocol": "TCP",
	}

	err = unstructured.SetNestedField(listener, "All", "allowedRoutes", "namespaces", "from")

	if err != nil {
		return fmt.Errorf("setting allowedRoutes.namespaces: %w", err)
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

	err := unstructured.SetNestedSlice(tcpRoute.Object, []any{
		map[string]any{
			"group":       "gateway.networking.x-k8s.io",
			"kind":        "XListenerSet",
			"namespace":   gatewayNamespace,
			"name":        name,
			"sectionName": sshListenerName,
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
					"name":      sshServiceName,
					"namespace": instanceNs,
					"port":      int64(sshPort),
				},
			},
		},
	}, "spec", "rules")

	if err != nil {
		return fmt.Errorf("setting rules: %w", err)
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

	err := unstructured.SetNestedSlice(refGrant.Object, []any{
		map[string]any{
			"group":     "gateway.networking.k8s.io",
			"kind":      "TCPRoute",
			"namespace": gatewayNamespace,
		},
	}, "spec", "from")

	if err != nil {
		return fmt.Errorf("setting from: %w", err)
	}

	err = unstructured.SetNestedSlice(refGrant.Object, []any{
		map[string]any{
			"group": "",
			"kind":  "Service",
			"name":  sshServiceName,
		},
	}, "spec", "to")

	if err != nil {
		return fmt.Errorf("setting to: %w", err)
	}

	return svc.SetDesiredKubeObject(refGrant, name+"-refgrant")
}

func createGatewayNetworkPolicy(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo, name, gatewayNamespace string) error {
	instanceNs := comp.GetInstanceNamespace()

	protocol := corev1.ProtocolTCP
	port := intstr.FromInt(sshPort)

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
					"app.kubernetes.io/instance": comp.GetName(),
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

	state.port = utils.ToInt32(listenerMap["port"])

	if state.port == 0 {
		return state
	}

	svc.Log.Info("Observed allocated SSH port", "port", state.port, "gateway", state.gatewayName)

	return state
}

func defaultGatewayName(svc *runtime.ServiceRuntime) string {
	raw, ok := svc.Config.Data["sshGateways"]
	if !ok || raw == "" {
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
