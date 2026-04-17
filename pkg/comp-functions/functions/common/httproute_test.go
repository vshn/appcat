package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	httpGatewayName      = "http-gateway"
	httpGatewayNamespace = "syn-kgateway"
)

func TestGenerateHTTPRoute(t *testing.T) {
	t.Run("GivenSingleFQDN_ExpectHTTPRouteInInstanceNs", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		route, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs: []string{"my-domain.com"},
			ServiceConfig: IngressRuleConfig{
				ServiceNameSuffix: svcNameSuffix,
				ServicePortNumber: 443,
			},
			GatewayName:      httpGatewayName,
			GatewayNamespace: httpGatewayNamespace,
		})

		assert.NoError(t, err)
		assert.Equal(t, compName+"-httproute", route.Name)
		assert.Equal(t, "vshn-"+compName, route.Namespace, "HTTPRoute must be in instance ns")

		assert.Len(t, route.Spec.ParentRefs, 1)
		pr := route.Spec.ParentRefs[0]
		assert.NotNil(t, pr.Group)
		assert.Equal(t, gatewayv1.Group("gateway.networking.x-k8s.io"), *pr.Group)
		assert.NotNil(t, pr.Kind)
		assert.Equal(t, gatewayv1.Kind("XListenerSet"), *pr.Kind)
		assert.Equal(t, gatewayv1.ObjectName(compName+"-listenerset"), pr.Name)
		assert.Nil(t, pr.Namespace, "parentRef must have no Namespace (same-ns)")

		assert.Len(t, route.Spec.Hostnames, 1)
		assert.Equal(t, gatewayv1.Hostname("my-domain.com"), route.Spec.Hostnames[0])

		assert.Len(t, route.Spec.Rules, 1)
		assert.Len(t, route.Spec.Rules[0].BackendRefs, 1)
		be := route.Spec.Rules[0].BackendRefs[0]
		assert.Equal(t, gatewayv1.ObjectName(compName+"-"+svcNameSuffix), be.Name)
		assert.Nil(t, be.Namespace, "backendRef must have no Namespace (same-ns)")
		assert.Equal(t, gatewayv1.PortNumber(443), *be.Port)
	})

	t.Run("GivenMultipleFQDNs_ExpectAllHostnames", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		route, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs: []string{"a.example.com", "b.example.com", "c.example.com"},
			ServiceConfig: IngressRuleConfig{
				ServiceNameSuffix: svcNameSuffix,
				ServicePortNumber: 8080,
			},
			GatewayName:      httpGatewayName,
			GatewayNamespace: httpGatewayNamespace,
		})
		assert.NoError(t, err)
		assert.Len(t, route.Spec.Hostnames, 3)
	})

	t.Run("GivenNameSuffix_ExpectSuffixedNames", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		route, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs:      []string{"collabora.example.com"},
			NameSuffix: "collabora-code",
			ServiceConfig: IngressRuleConfig{
				ServiceNameSuffix: "collabora-code",
				ServicePortNumber: 9980,
			},
			GatewayName:      httpGatewayName,
			GatewayNamespace: httpGatewayNamespace,
		})
		assert.NoError(t, err)
		assert.Equal(t, compName+"-collabora-code-httproute", route.Name)
		assert.Equal(t, gatewayv1.ObjectName(compName+"-collabora-code-listenerset"), route.Spec.ParentRefs[0].Name)
	})

	t.Run("GivenNoFQDNs_ExpectError", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}
		_, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs:            []string{},
			ServiceConfig:    IngressRuleConfig{ServicePortNumber: 8080},
			GatewayName:      httpGatewayName,
			GatewayNamespace: httpGatewayNamespace,
		})
		assert.Error(t, err)
	})

	t.Run("GivenEmptyFQDN_ExpectError", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}
		_, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs:            []string{""},
			ServiceConfig:    IngressRuleConfig{ServicePortNumber: 8080},
			GatewayName:      httpGatewayName,
			GatewayNamespace: httpGatewayNamespace,
		})
		assert.Error(t, err)
	})

	t.Run("GivenZeroPort_ExpectError", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}
		_, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs:            []string{"example.com"},
			ServiceConfig:    IngressRuleConfig{ServiceNameSuffix: svcNameSuffix},
			GatewayName:      httpGatewayName,
			GatewayNamespace: httpGatewayNamespace,
		})
		assert.Error(t, err)
	})

	t.Run("GivenTooLongName_ExpectSanitizedNames", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		longName := "looooonggloooooonggggmaaaaaaaaaaaaaan" // 37 chars
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: longName}}
		route, err := GenerateHTTPRoute(comp, svc, HTTPRouteConfig{
			FQDNs:            []string{"example.com"},
			NameSuffix:       "collabora-code",
			ServiceConfig:    IngressRuleConfig{ServiceNameSuffix: "x", ServicePortNumber: 80},
			GatewayName:      httpGatewayName,
			GatewayNamespace: httpGatewayNamespace,
		})
		assert.NoError(t, err)
		assert.LessOrEqual(t, len(route.Name), 63)
		assert.LessOrEqual(t, len(string(route.Spec.ParentRefs[0].Name)), 63)
		assert.LessOrEqual(t, len(string(route.Spec.Rules[0].BackendRefs[0].Name)), 63)
	})
}

func TestGenerateXListenerSet(t *testing.T) {
	t.Run("GivenSingleFQDN_ExpectXListenerSetInInstanceNs", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		ls, err := GenerateXListenerSet(comp, svc, HTTPRouteConfig{
			FQDNs: []string{"my-domain.com"},
			ServiceConfig: IngressRuleConfig{
				ServiceNameSuffix: svcNameSuffix,
				ServicePortNumber: 443,
			},
			GatewayName:      httpGatewayName,
			GatewayNamespace: httpGatewayNamespace,
		})
		assert.NoError(t, err)
		assert.Equal(t, "gateway.networking.x-k8s.io/v1alpha1", ls.GetAPIVersion())
		assert.Equal(t, "XListenerSet", ls.GetKind())
		assert.Equal(t, compName+"-listenerset", ls.GetName())
		assert.Equal(t, "vshn-"+compName, ls.GetNamespace())

		annos := ls.GetAnnotations()
		assert.Equal(t, "letsencrypt-production", annos["cert-manager.io/cluster-issuer"])

		parentRef, found, err := unstructuredNestedMap(ls.Object, "spec", "parentRef")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "gateway.networking.k8s.io", parentRef["group"])
		assert.Equal(t, "Gateway", parentRef["kind"])
		assert.Equal(t, httpGatewayName, parentRef["name"])
		assert.Equal(t, httpGatewayNamespace, parentRef["namespace"])

		listeners, found, err := unstructuredNestedSlice(ls.Object, "spec", "listeners")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Len(t, listeners, 1)
		l := listeners[0].(map[string]any)
		assert.Equal(t, "my-domain.com", l["hostname"])
		assert.Equal(t, int64(443), l["port"])
		assert.Equal(t, "HTTPS", l["protocol"])
		tls := l["tls"].(map[string]any)
		assert.Equal(t, "Terminate", tls["mode"])
		certRefs := tls["certificateRefs"].([]any)
		assert.Len(t, certRefs, 1)
		certRef := certRefs[0].(map[string]any)
		secretName := certRef["name"].(string)
		assert.True(t, len(secretName) <= 63)
		assert.True(t, len(secretName) > len(compName)+4)
		assert.True(t, len(l["name"].(string)) == 10)
		assert.Equal(t, "l-", l["name"].(string)[:2])
	})

	t.Run("GivenThreeFQDNs_ExpectThreeListeners", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		ls, err := GenerateXListenerSet(comp, svc, HTTPRouteConfig{
			FQDNs: []string{"a.example.com", "b.example.com", "c.example.com"},
			ServiceConfig: IngressRuleConfig{
				ServiceNameSuffix: svcNameSuffix,
				ServicePortNumber: 443,
			},
			GatewayName:      httpGatewayName,
			GatewayNamespace: httpGatewayNamespace,
		})
		assert.NoError(t, err)
		listeners, _, _ := unstructuredNestedSlice(ls.Object, "spec", "listeners")
		assert.Len(t, listeners, 3)
	})

	t.Run("GivenNameSuffix_ExpectSuffixedName", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}
		ls, err := GenerateXListenerSet(comp, svc, HTTPRouteConfig{
			FQDNs:            []string{"collabora.example.com"},
			NameSuffix:       "collabora-code",
			ServiceConfig:    IngressRuleConfig{ServiceNameSuffix: "collabora-code", ServicePortNumber: 9980},
			GatewayName:      httpGatewayName,
			GatewayNamespace: httpGatewayNamespace,
		})
		assert.NoError(t, err)
		assert.Equal(t, compName+"-collabora-code-listenerset", ls.GetName())
	})

	t.Run("GivenMissingGatewayName_ExpectError", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}
		_, err := GenerateXListenerSet(comp, svc, HTTPRouteConfig{
			FQDNs:            []string{"example.com"},
			ServiceConfig:    IngressRuleConfig{ServiceNameSuffix: "x", ServicePortNumber: 80},
			GatewayNamespace: httpGatewayNamespace,
		})
		assert.Error(t, err)
	})

	t.Run("GivenTooLongName_ExpectSanitizedName", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		longName := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: longName}}
		ls, err := GenerateXListenerSet(comp, svc, HTTPRouteConfig{
			FQDNs:            []string{"example.com"},
			NameSuffix:       "collabora-code",
			ServiceConfig:    IngressRuleConfig{ServiceNameSuffix: "x", ServicePortNumber: 80},
			GatewayName:      httpGatewayName,
			GatewayNamespace: httpGatewayNamespace,
		})
		assert.NoError(t, err)
		assert.LessOrEqual(t, len(ls.GetName()), 63)
	})
}

func TestGenerateCertificates(t *testing.T) {
	cfg := HTTPRouteConfig{
		FQDNs:            []string{"a.example.com", "b.example.com"},
		ServiceConfig:    IngressRuleConfig{ServiceNameSuffix: svcNameSuffix, ServicePortNumber: 443},
		GatewayName:      httpGatewayName,
		GatewayNamespace: httpGatewayNamespace,
	}

	t.Run("GivenClusterIssuerAnnotation_ExpectCertificatePerFQDN", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		certs, err := GenerateCertificates(comp, svc, cfg)
		assert.NoError(t, err)
		assert.Len(t, certs, 2)

		for i, fqdn := range cfg.FQDNs {
			c := certs[i]
			wantSecret := tlsSecretName(comp, fqdn)
			assert.Equal(t, wantSecret, c.Name)
			assert.Equal(t, wantSecret, c.Spec.SecretName)
			assert.Equal(t, "vshn-"+compName, c.Namespace)
			assert.Equal(t, []string{fqdn}, c.Spec.DNSNames)
			assert.Equal(t, "letsencrypt-production", c.Spec.IssuerRef.Name)
			assert.Equal(t, "ClusterIssuer", c.Spec.IssuerRef.Kind)
			assert.Equal(t, "cert-manager.io", c.Spec.IssuerRef.Group)
		}
	})

	t.Run("GivenNoIssuerAnnotation_ExpectNilCerts", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		svc.Config.Data["ingress_annotations"] = ""
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		certs, err := GenerateCertificates(comp, svc, cfg)
		assert.NoError(t, err)
		assert.Nil(t, certs)
	})

	t.Run("GivenNamespacedIssuerAnnotation_ExpectIssuerKind", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		svc.Config.Data["ingress_annotations"] = "cert-manager.io/issuer: my-issuer"
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		certs, err := GenerateCertificates(comp, svc, cfg)
		assert.NoError(t, err)
		assert.Len(t, certs, 2)
		assert.Equal(t, "my-issuer", certs[0].Spec.IssuerRef.Name)
		assert.Equal(t, "Issuer", certs[0].Spec.IssuerRef.Kind)
	})

	t.Run("GivenInvalidConfig_ExpectError", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/03_httproute.yaml")
		comp := &baaSComposite{ObjectMeta: metav1.ObjectMeta{Name: compName}}

		_, err := GenerateCertificates(comp, svc, HTTPRouteConfig{
			ServiceConfig:    IngressRuleConfig{ServiceNameSuffix: svcNameSuffix, ServicePortNumber: 443},
			GatewayName:      httpGatewayName,
			GatewayNamespace: httpGatewayNamespace,
		})
		assert.Error(t, err)
	})
}

func unstructuredNestedMap(obj map[string]any, keys ...string) (map[string]any, bool, error) {
	var cur any = obj
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("key %q: parent is not map[string]any", k)
		}
		v, ok := m[k]
		if !ok {
			return nil, false, nil
		}
		cur = v
	}
	result, ok := cur.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("terminal value is not map[string]any")
	}
	return result, true, nil
}

func unstructuredNestedSlice(obj map[string]any, keys ...string) ([]any, bool, error) {
	var cur any = obj
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("key %q: parent is not map[string]any", k)
		}
		v, ok := m[k]
		if !ok {
			return nil, false, nil
		}
		cur = v
	}
	result, ok := cur.([]any)
	if !ok {
		return nil, false, fmt.Errorf("terminal value is not []any")
	}
	return result, true, nil
}
