package common

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	clusterIssuerAnnotation = "cert-manager.io/cluster-issuer"
	issuerAnnotation        = "cert-manager.io/issuer"
	issuerKindAnnotation    = "cert-manager.io/issuer-kind"
	issuerGroupAnnotation   = "cert-manager.io/issuer-group"
)

const (
	xListenerSetGroup   = "gateway.networking.x-k8s.io"
	xListenerSetVersion = "v1alpha1"
	xListenerSetKind    = "XListenerSet"

	gatewayGroup = "gateway.networking.k8s.io"
	gatewayKind  = "Gateway"

	maxK8sNameLen = 63

	// RouteTypeHTTPRoute is the svc.Config.Data["routeType"] value that
	// selects Gateway API HTTPRoute over classic Ingress.
	RouteTypeHTTPRoute = "HTTPRoute"
)

// IsHTTPRouteMode reports whether the composition is configured to render
// HTTPRoute/XListenerSet objects instead of Ingress.
func IsHTTPRouteMode(svc *runtime.ServiceRuntime) bool {
	return svc.Config.Data["routeType"] == RouteTypeHTTPRoute
}

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
		return fmt.Errorf("ServicePortNumber is required for HTTPRoute")
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
	return runtime.EscapeDNS1123Label(compBaseName(comp, cfg)+"-httproute", maxK8sNameLen)
}

func listenerSetName(comp InfoGetter, cfg HTTPRouteConfig) string {
	return runtime.EscapeDNS1123Label(compBaseName(comp, cfg)+"-listenerset", maxK8sNameLen)
}

func fqdnHash8(fqdn string) string {
	sum := sha256.Sum256([]byte(fqdn))
	return hex.EncodeToString(sum[:])[:8]
}

func tlsSecretName(comp InfoGetter, fqdn string) string {
	return runtime.EscapeDNS1123Label(fmt.Sprintf("%s-%s-tls", comp.GetName(), fqdnHash8(fqdn)), maxK8sNameLen)
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
	lsName := listenerSetName(comp, cfg)

	svcNameSuffix := cfg.ServiceConfig.ServiceNameSuffix
	if !strings.HasPrefix(svcNameSuffix, "-") && len(svcNameSuffix) > 0 {
		svcNameSuffix = "-" + svcNameSuffix
	}
	serviceName := runtime.EscapeDNS1123Label(comp.GetName()+svcNameSuffix, maxK8sNameLen)

	hostnames := make([]gatewayv1.Hostname, len(cfg.FQDNs))
	for i, fqdn := range cfg.FQDNs {
		hostnames[i] = gatewayv1.Hostname(fqdn)
	}

	parentGroup := gatewayv1.Group(xListenerSetGroup)
	parentKind := gatewayv1.Kind(xListenerSetKind)
	port := gatewayv1.PortNumber(cfg.ServiceConfig.ServicePortNumber)

	labels := getIngressLabels(svc, cfg.AdditionalLabels)

	relPath := cfg.ServiceConfig.RelPath
	if relPath == "" {
		relPath = "/"
	}
	pathMatchType := gatewayv1.PathMatchPathPrefix
	matches := []gatewayv1.HTTPRouteMatch{
		{
			Path: &gatewayv1.HTTPPathMatch{
				Type:  &pathMatchType,
				Value: &relPath,
			},
		},
	}

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
					Matches: matches,
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
// attached to the shared Gateway. TLS secrets per listener are produced by
// GenerateCertificates (see that func for why ingress_annotations aren't
// propagated here).
func GenerateXListenerSet(comp InfoGetter, svc *runtime.ServiceRuntime, cfg HTTPRouteConfig) (*unstructured.Unstructured, error) {
	if err := validateHTTPRouteConfig(cfg); err != nil {
		return nil, err
	}

	lsName := listenerSetName(comp, cfg)

	listeners := make([]any, 0, len(cfg.FQDNs))
	for _, fqdn := range cfg.FQDNs {
		secretName := tlsSecretName(comp, fqdn)
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

	labels := getIngressLabels(svc, cfg.AdditionalLabels)

	ls := &unstructured.Unstructured{}
	ls.SetAPIVersion(xListenerSetGroup + "/" + xListenerSetVersion)
	ls.SetKind(xListenerSetKind)
	ls.SetName(lsName)
	ls.SetNamespace(comp.GetInstanceNamespace())
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

// issuerRefFromAnnotations extracts a cert-manager issuerRef from the
// ingress_annotations composition config. Returns nil if no cert-manager
// issuer annotation is set.
func issuerRefFromAnnotations(annotations map[string]string) *cmmetav1.ObjectReference {
	if name, ok := annotations[clusterIssuerAnnotation]; ok && name != "" {
		return &cmmetav1.ObjectReference{
			Name:  name,
			Kind:  "ClusterIssuer",
			Group: "cert-manager.io",
		}
	}
	if name, ok := annotations[issuerAnnotation]; ok && name != "" {
		kind := "Issuer"
		if v, ok := annotations[issuerKindAnnotation]; ok && v != "" {
			kind = v
		}
		group := "cert-manager.io"
		if v, ok := annotations[issuerGroupAnnotation]; ok && v != "" {
			group = v
		}
		return &cmmetav1.ObjectReference{Name: name, Kind: kind, Group: group}
	}
	return nil
}

// GenerateCertificates creates cert-manager Certificate resources, one per
// FQDN, with secret names matching the listeners produced by
// GenerateXListenerSet. Returns (nil, nil) if no cert-manager issuer annotation
// is configured (in that case the user is expected to provide the TLS secret
// themselves). cert-manager's Gateway API shim does not yet understand
// XListenerSet, so we create Certificates explicitly instead of relying on
// annotation propagation.
func GenerateCertificates(comp InfoGetter, svc *runtime.ServiceRuntime, cfg HTTPRouteConfig) ([]*cmv1.Certificate, error) {
	if err := validateHTTPRouteConfig(cfg); err != nil {
		return nil, err
	}

	issuerRef := issuerRefFromAnnotations(getIngressAnnotations(svc, nil))
	if issuerRef == nil {
		return nil, nil
	}

	labels := getIngressLabels(svc, cfg.AdditionalLabels)
	certs := make([]*cmv1.Certificate, 0, len(cfg.FQDNs))
	for _, fqdn := range cfg.FQDNs {
		secretName := tlsSecretName(comp, fqdn)
		certs = append(certs, &cmv1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: comp.GetInstanceNamespace(),
				Labels:    labels,
			},
			Spec: cmv1.CertificateSpec{
				SecretName: secretName,
				DNSNames:   []string{fqdn},
				IssuerRef:  *issuerRef,
			},
		})
	}
	return certs, nil
}

// CreateCertificates applies generated Certificates using svc.SetDesiredKubeObject().
func CreateCertificates(svc *runtime.ServiceRuntime, certs []*cmv1.Certificate, opts ...runtime.KubeObjectOption) error {
	for _, c := range certs {
		if err := svc.SetDesiredKubeObject(c, c.Name, opts...); err != nil {
			return err
		}
	}
	return nil
}

// ErrHTTPGatewayNotConfigured signals a pipeline misconfig — callers
// returning *xfnproto.Result should surface it as Fatal.
var ErrHTTPGatewayNotConfigured = errors.New("httpGatewayName and httpGatewayNamespace must be set when routeType=HTTPRoute")

// ApplyHTTPRoute applies the XListenerSet, HTTPRoute, and Certificates for
// comp. cfg.GatewayName/Namespace default to svc.Config.Data if empty.
func ApplyHTTPRoute(comp InfoGetter, svc *runtime.ServiceRuntime, cfg HTTPRouteConfig, opts ...runtime.KubeObjectOption) error {
	if cfg.GatewayName == "" {
		cfg.GatewayName = svc.Config.Data["httpGatewayName"]
	}
	if cfg.GatewayNamespace == "" {
		cfg.GatewayNamespace = svc.Config.Data["httpGatewayNamespace"]
	}
	if cfg.GatewayName == "" || cfg.GatewayNamespace == "" {
		return ErrHTTPGatewayNotConfigured
	}

	ls, err := GenerateXListenerSet(comp, svc, cfg)
	if err != nil {
		return fmt.Errorf("cannot generate XListenerSet: %w", err)
	}
	if err := CreateXListenerSets(svc, []*unstructured.Unstructured{ls}, opts...); err != nil {
		return fmt.Errorf("cannot create XListenerSet: %w", err)
	}

	route, err := GenerateHTTPRoute(comp, svc, cfg)
	if err != nil {
		return fmt.Errorf("cannot generate HTTPRoute: %w", err)
	}
	if err := CreateHTTPRoutes(svc, []*gatewayv1.HTTPRoute{route}, opts...); err != nil {
		return fmt.Errorf("cannot create HTTPRoute: %w", err)
	}

	certs, err := GenerateCertificates(comp, svc, cfg)
	if err != nil {
		return fmt.Errorf("cannot generate Certificates: %w", err)
	}
	if err := CreateCertificates(svc, certs, opts...); err != nil {
		return fmt.Errorf("cannot create Certificates: %w", err)
	}
	return nil
}

// ApplyHTTPRouteAsResult is ApplyHTTPRoute wrapped as *xfnproto.Result:
// Fatal on ErrHTTPGatewayNotConfigured, Warning otherwise, nil on success.
func ApplyHTTPRouteAsResult(comp InfoGetter, svc *runtime.ServiceRuntime, cfg HTTPRouteConfig, opts ...runtime.KubeObjectOption) *xfnproto.Result {
	err := ApplyHTTPRoute(comp, svc, cfg, opts...)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrHTTPGatewayNotConfigured) {
		return runtime.NewFatalResult(err)
	}
	return runtime.NewWarningResult(err.Error())
}
