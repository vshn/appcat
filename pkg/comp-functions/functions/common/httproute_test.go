package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestGenerateHTTPRoute(t *testing.T) {
	t.Run("GivenSingleFQDN_ExpectHTTPRoute", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{
			ObjectMeta: metav1.ObjectMeta{Name: compName},
		}

		route, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs: []string{"my-domain.com"},
			ServiceConfig: IngressRuleConfig{
				ServiceNameSuffix: svcNameSuffix,
				ServicePortNumber: 443,
			},
			GatewayName:      "http-gateway",
			GatewayNamespace: "syn-kgateway",
		})

		assert.NoError(t, err)
		assert.Equal(t, compName+"-httproute", route.Name)
		assert.Equal(t, "syn-kgateway", route.Namespace)
		assert.Len(t, route.Spec.Hostnames, 1)
		assert.Equal(t, gatewayv1.Hostname("my-domain.com"), route.Spec.Hostnames[0])
		assert.Len(t, route.Spec.ParentRefs, 1)
		assert.Equal(t, gatewayv1.ObjectName("http-gateway"), route.Spec.ParentRefs[0].Name)
		assert.Len(t, route.Spec.Rules, 1)
		assert.Len(t, route.Spec.Rules[0].BackendRefs, 1)

		backendRef := route.Spec.Rules[0].BackendRefs[0]
		assert.Equal(t, gatewayv1.ObjectName(compName+"-"+svcNameSuffix), backendRef.Name)
		ns := gatewayv1.Namespace("vshn-" + compName)
		assert.Equal(t, &ns, backendRef.Namespace)
	})

	t.Run("GivenMultipleFQDNs_ExpectAllHostnames", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{
			ObjectMeta: metav1.ObjectMeta{Name: compName},
		}

		route, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs: []string{"a.example.com", "b.example.com", "c.example.com"},
			ServiceConfig: IngressRuleConfig{
				ServiceNameSuffix: svcNameSuffix,
				ServicePortNumber: 8080,
			},
			GatewayName:      "http-gateway",
			GatewayNamespace: "syn-kgateway",
		})

		assert.NoError(t, err)
		assert.Len(t, route.Spec.Hostnames, 3)
		assert.Equal(t, gatewayv1.Hostname("a.example.com"), route.Spec.Hostnames[0])
		assert.Equal(t, gatewayv1.Hostname("b.example.com"), route.Spec.Hostnames[1])
		assert.Equal(t, gatewayv1.Hostname("c.example.com"), route.Spec.Hostnames[2])

		backendRef := route.Spec.Rules[0].BackendRefs[0]
		assert.Equal(t, gatewayv1.PortNumber(8080), *backendRef.Port)
	})

	t.Run("GivenNoFQDNs_ExpectError", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		_, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs:            []string{},
			ServiceConfig:    IngressRuleConfig{ServicePortNumber: 8080},
			GatewayName:      "http-gateway",
			GatewayNamespace: "syn-kgateway",
		})
		assert.Error(t, err)
	})

	t.Run("GivenEmptyFQDN_ExpectError", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		_, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs:            []string{""},
			ServiceConfig:    IngressRuleConfig{ServicePortNumber: 8080},
			GatewayName:      "http-gateway",
			GatewayNamespace: "syn-kgateway",
		})
		assert.Error(t, err)
	})

	t.Run("GivenPortNameAndNumber_ExpectPortNumberUsed", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		route, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs: []string{"example.com"},
			ServiceConfig: IngressRuleConfig{
				ServiceNameSuffix: svcNameSuffix,
				ServicePortName:   svcBackendPortName,
				ServicePortNumber: 1337,
			},
			GatewayName:      "http-gateway",
			GatewayNamespace: "syn-kgateway",
		})
		assert.NoError(t, err)
		assert.Equal(t, gatewayv1.PortNumber(1337), *route.Spec.Rules[0].BackendRefs[0].Port)
	})

	t.Run("GivenNeitherPortNameNorNumber_ExpectError", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		_, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs:            []string{"example.com"},
			ServiceConfig:    IngressRuleConfig{ServiceNameSuffix: svcNameSuffix},
			GatewayName:      "http-gateway",
			GatewayNamespace: "syn-kgateway",
		})
		assert.Error(t, err)
	})

	t.Run("GivenNameSuffix_ExpectSuffixInName", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		route, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs:            []string{"example.com"},
			ServiceConfig:    IngressRuleConfig{ServiceNameSuffix: svcNameSuffix, ServicePortNumber: 8080},
			GatewayName:      "http-gateway",
			GatewayNamespace: "syn-kgateway",
			NameSuffix:       "collabora-code",
		})
		assert.NoError(t, err)
		assert.Equal(t, compName+"-collabora-code-httproute", route.Name)
	})

	t.Run("GivenEmptyGatewayName_ExpectError", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		_, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs:            []string{"example.com"},
			ServiceConfig:    IngressRuleConfig{ServicePortNumber: 8080},
			GatewayName:      "",
			GatewayNamespace: "syn-kgateway",
		})
		assert.Error(t, err)
	})

	t.Run("GivenEmptyGatewayNamespace_ExpectError", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		_, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs:            []string{"example.com"},
			ServiceConfig:    IngressRuleConfig{ServicePortNumber: 8080},
			GatewayName:      "http-gateway",
			GatewayNamespace: "",
		})
		assert.Error(t, err)
	})
}

func TestGenerateReferenceGrant(t *testing.T) {
	t.Run("GivenValidInputs_ExpectReferenceGrant", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		grant, err := GenerateReferenceGrant(comp, svc, "syn-kgateway", compName+"-"+svcNameSuffix)
		assert.NoError(t, err)
		assert.Equal(t, compName+"-httpgrant", grant.Name)
		assert.Equal(t, "vshn-"+compName, grant.Namespace)

		assert.Len(t, grant.Spec.From, 1)
		assert.Equal(t, gatewayv1.Namespace("syn-kgateway"), grant.Spec.From[0].Namespace)

		assert.Len(t, grant.Spec.To, 1)
		svcName := gatewayv1.ObjectName(compName + "-" + svcNameSuffix)
		assert.Equal(t, &svcName, grant.Spec.To[0].Name)
	})

	t.Run("GivenNameSuffix_ExpectSuffixInName", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		grant, err := GenerateReferenceGrant(comp, svc, "syn-kgateway", "my-svc", "collabora-code")
		assert.NoError(t, err)
		assert.Equal(t, compName+"-collabora-code-httpgrant", grant.Name)
	})
}
