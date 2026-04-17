package common

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	xListenerSetGroup   = "gateway.networking.x-k8s.io"
	xListenerSetVersion = "v1alpha1"
	xListenerSetKind    = "XListenerSet"

	gatewayGroup = "gateway.networking.k8s.io"
	gatewayKind  = "Gateway"

	maxK8sNameLen = 63
)

// HTTPRouteConfig contains configuration for generating an HTTPRoute and its
// accompanying XListenerSet.
type HTTPRouteConfig struct {
	AdditionalLabels map[string]string
	FQDNs            []string
	GatewayName      string
	GatewayNamespace string
	NameSuffix       string
	ServiceConfig    IngressRuleConfig
}

func validateHTTPRouteConfig(cfg HTTPRouteConfig) error {
	if len(cfg.FQDNs) == 0 {
		return fmt.Errorf("no FQDNs have been defined")
	}
	for _, f := range cfg.FQDNs {
		if f == "" {
			return fmt.Errorf("an empty FQDN has been passed, FQDNs: %v", cfg.FQDNs)
		}
	}
	if cfg.ServiceConfig.ServicePortNumber == 0 {
		return fmt.Errorf("ServicePortNumber is required for HTTPRoute (set an explicit port number; Gateway API does not resolve Service port names, unlike Ingress)")
	}
	if cfg.GatewayName == "" || cfg.GatewayNamespace == "" {
		return fmt.Errorf("GatewayName and GatewayNamespace must be set")
	}
	return nil
}

// compBaseName returns the shared prefix used for the HTTPRoute, XListenerSet,
// and any other per-composite Gateway API objects: "<comp>[-<nameSuffix>]".
func compBaseName(comp InfoGetter, cfg HTTPRouteConfig) string {
	name := comp.GetName()
	if cfg.NameSuffix != "" {
		name = name + "-" + strings.Trim(cfg.NameSuffix, "-")
	}
	return name
}

func routeName(comp InfoGetter, cfg HTTPRouteConfig) string {
	return compBaseName(comp, cfg) + "-httproute"
}

func listenerSetName(comp InfoGetter, cfg HTTPRouteConfig) string {
	return compBaseName(comp, cfg) + "-listenerset"
}

func fqdnHash8(fqdn string) string {
	sum := sha256.Sum256([]byte(fqdn))
	return hex.EncodeToString(sum[:])[:8]
}

func tlsSecretName(comp InfoGetter, fqdn string) string {
	return fmt.Sprintf("%s-%s-tls", comp.GetName(), fqdnHash8(fqdn))
}

func listenerName(fqdn string) string {
	return "l-" + fqdnHash8(fqdn)
}

// GenerateHTTPRoute creates an HTTPRoute in the instance namespace, parented to
// the XListenerSet (same ns) so no cross-namespace ref is needed.
func GenerateHTTPRoute(comp InfoGetter, svc *runtime.ServiceRuntime, cfg HTTPRouteConfig) (*gatewayv1.HTTPRoute, error) {
	if err := validateHTTPRouteConfig(cfg); err != nil {
		return nil, err
	}

	rName := routeName(comp, cfg)
	if len(rName) > maxK8sNameLen {
		return nil, fmt.Errorf("generated HTTPRoute name %q exceeds %d chars", rName, maxK8sNameLen)
	}
	lsName := listenerSetName(comp, cfg)
	if len(lsName) > maxK8sNameLen {
		return nil, fmt.Errorf("generated XListenerSet name %q exceeds %d chars", lsName, maxK8sNameLen)
	}

	svcNameSuffix := cfg.ServiceConfig.ServiceNameSuffix
	if !strings.HasPrefix(svcNameSuffix, "-") && len(svcNameSuffix) > 0 {
		svcNameSuffix = "-" + svcNameSuffix
	}
	serviceName := comp.GetName() + svcNameSuffix
	if len(serviceName) > maxK8sNameLen {
		return nil, fmt.Errorf("generated backend Service name %q exceeds %d chars", serviceName, maxK8sNameLen)
	}

	hostnames := make([]gatewayv1.Hostname, len(cfg.FQDNs))
	for i, fqdn := range cfg.FQDNs {
		hostnames[i] = gatewayv1.Hostname(fqdn)
	}

	parentGroup := gatewayv1.Group(xListenerSetGroup)
	parentKind := gatewayv1.Kind(xListenerSetKind)
	port := gatewayv1.PortNumber(cfg.ServiceConfig.ServicePortNumber)

	labels := getIngressLabels(svc, cfg.AdditionalLabels)

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rName,
			Namespace: comp.GetInstanceNamespace(),
			Labels:    labels,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Group: &parentGroup,
						Kind:  &parentKind,
						Name:  gatewayv1.ObjectName(lsName),
					},
				},
			},
			Hostnames: hostnames,
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(serviceName),
									Port: &port,
								},
							},
						},
					},
				},
			},
		},
	}
	return route, nil
}

// GenerateXListenerSet creates an XListenerSet in the instance namespace,
// attached to the shared Gateway. Annotations from ingress_annotations propagate
// so cert-manager issues a Certificate per listener hostname.
func GenerateXListenerSet(comp InfoGetter, svc *runtime.ServiceRuntime, cfg HTTPRouteConfig) (*unstructured.Unstructured, error) {
	if err := validateHTTPRouteConfig(cfg); err != nil {
		return nil, err
	}

	lsName := listenerSetName(comp, cfg)
	if len(lsName) > maxK8sNameLen {
		return nil, fmt.Errorf("generated XListenerSet name %q exceeds %d chars", lsName, maxK8sNameLen)
	}

	listeners := make([]any, 0, len(cfg.FQDNs))
	for _, fqdn := range cfg.FQDNs {
		secretName := tlsSecretName(comp, fqdn)
		if len(secretName) > maxK8sNameLen {
			return nil, fmt.Errorf("generated TLS secret name %q exceeds %d chars", secretName, maxK8sNameLen)
		}
		listeners = append(listeners, map[string]any{
			"name":     listenerName(fqdn),
			"hostname": fqdn,
			"port":     int64(443),
			"protocol": "HTTPS",
			"tls": map[string]any{
				"mode": "Terminate",
				"certificateRefs": []any{
					map[string]any{"name": secretName},
				},
			},
			"allowedRoutes": map[string]any{
				"namespaces": map[string]any{"from": "Same"},
			},
		})
	}

	annotations := getIngressAnnotations(svc, nil)
	labels := getIngressLabels(svc, cfg.AdditionalLabels)

	ls := &unstructured.Unstructured{}
	ls.SetAPIVersion(xListenerSetGroup + "/" + xListenerSetVersion)
	ls.SetKind(xListenerSetKind)
	ls.SetName(lsName)
	ls.SetNamespace(comp.GetInstanceNamespace())
	if len(annotations) > 0 {
		ls.SetAnnotations(annotations)
	}
	if len(labels) > 0 {
		ls.SetLabels(labels)
	}
	ls.Object["spec"] = map[string]any{
		"parentRef": map[string]any{
			"group":     gatewayGroup,
			"kind":      gatewayKind,
			"name":      cfg.GatewayName,
			"namespace": cfg.GatewayNamespace,
		},
		"listeners": listeners,
	}
	return ls, nil
}

// CreateHTTPRoutes applies generated HTTPRoutes using svc.SetDesiredKubeObject().
func CreateHTTPRoutes(svc *runtime.ServiceRuntime, routes []*gatewayv1.HTTPRoute, opts ...runtime.KubeObjectOption) error {
	for _, route := range routes {
		err := svc.SetDesiredKubeObject(route, route.Name, opts...)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateXListenerSets applies generated XListenerSets using svc.SetDesiredKubeObject().
func CreateXListenerSets(svc *runtime.ServiceRuntime, sets []*unstructured.Unstructured, opts ...runtime.KubeObjectOption) error {
	for _, ls := range sets {
		err := svc.SetDesiredKubeObject(ls, ls.GetName(), opts...)
		if err != nil {
			return err
		}
	}
	return nil
}
