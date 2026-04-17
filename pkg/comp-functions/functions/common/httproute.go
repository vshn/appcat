package common

import (
	"fmt"
	"strings"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// HTTPRouteConfig contains configuration for generating an HTTPRoute object.
type HTTPRouteConfig struct {
	AdditionalLabels map[string]string // Optional
	FQDNs            []string
	GatewayName      string
	GatewayNamespace string
	NameSuffix       string // Optional, appended before "-httproute" (e.g. "collabora-code")
	ServiceConfig    IngressRuleConfig
}

// GenerateHTTPRoute creates a single HTTPRoute in the Gateway namespace with
// all FQDNs as hostnames and one rule pointing to the backend service in the
// instance namespace.
func GenerateHTTPRoute(comp InfoGetter, svc *runtime.ServiceRuntime, config HTTPRouteConfig) (*gatewayv1.HTTPRoute, error) {
	if len(config.FQDNs) == 0 {
		return nil, fmt.Errorf("no FQDNs have been defined")
	}
	for _, fqdn := range config.FQDNs {
		if fqdn == "" {
			return nil, fmt.Errorf("an empty FQDN has been passed, FQDNs: %v", config.FQDNs)
		}
	}

	if config.ServiceConfig.ServicePortNumber == 0 {
		return nil, fmt.Errorf("ServicePortNumber is required for HTTPRoute (Gateway API does not support port names)")
	}

	if config.GatewayName == "" || config.GatewayNamespace == "" {
		return nil, fmt.Errorf("GatewayName and GatewayNamespace must be set")
	}

	svcNameSuffix := config.ServiceConfig.ServiceNameSuffix
	if !strings.HasPrefix(svcNameSuffix, "-") && len(svcNameSuffix) > 0 {
		svcNameSuffix = "-" + svcNameSuffix
	}
	serviceName := comp.GetName() + svcNameSuffix

	hostnames := make([]gatewayv1.Hostname, len(config.FQDNs))
	for i, fqdn := range config.FQDNs {
		hostnames[i] = gatewayv1.Hostname(fqdn)
	}

	routeName := comp.GetName()
	if config.NameSuffix != "" {
		routeName = routeName + "-" + strings.Trim(config.NameSuffix, "-")
	}
	routeName = routeName + "-httproute"

	gwNamespace := gatewayv1.Namespace(config.GatewayNamespace)
	instanceNamespace := gatewayv1.Namespace(comp.GetInstanceNamespace())
	port := gatewayv1.PortNumber(config.ServiceConfig.ServicePortNumber)
	svcGroup := gatewayv1.Group("")
	svcKind := gatewayv1.Kind("Service")

	labels := getIngressLabels(svc, config.AdditionalLabels)

	route := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: config.GatewayNamespace,
			Labels:    labels,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Name:      gatewayv1.ObjectName(config.GatewayName),
						Namespace: &gwNamespace,
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
									Group:     &svcGroup,
									Kind:      &svcKind,
									Name:      gatewayv1.ObjectName(serviceName),
									Namespace: &instanceNamespace,
									Port:      &port,
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

// GenerateReferenceGrant creates a ReferenceGrant in the instance namespace
// that allows HTTPRoutes from the Gateway namespace to reference the named
// Service in the instance namespace.
func GenerateReferenceGrant(comp InfoGetter, svc *runtime.ServiceRuntime, gatewayNamespace string, serviceName string, nameSuffix ...string) (*gatewayv1beta1.ReferenceGrant, error) {
	if gatewayNamespace == "" {
		return nil, fmt.Errorf("gatewayNamespace must be set")
	}
	if serviceName == "" {
		return nil, fmt.Errorf("serviceName must be set")
	}

	grantName := comp.GetName()
	if len(nameSuffix) > 0 && nameSuffix[0] != "" {
		grantName = grantName + "-" + strings.Trim(nameSuffix[0], "-")
	}
	grantName = grantName + "-httpgrant"

	svcName := gatewayv1beta1.ObjectName(serviceName)

	grant := &gatewayv1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      grantName,
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1.Group("gateway.networking.k8s.io"),
					Kind:      "HTTPRoute",
					Namespace: gatewayv1.Namespace(gatewayNamespace),
				},
			},
			To: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: "",
					Kind:  "Service",
					Name:  &svcName,
				},
			},
		},
	}

	return grant, nil
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

// CreateReferenceGrants applies generated ReferenceGrants using svc.SetDesiredKubeObject().
func CreateReferenceGrants(svc *runtime.ServiceRuntime, grants []*gatewayv1beta1.ReferenceGrant, opts ...runtime.KubeObjectOption) error {
	for _, grant := range grants {
		err := svc.SetDesiredKubeObject(grant, grant.Name, opts...)
		if err != nil {
			return err
		}
	}
	return nil
}
