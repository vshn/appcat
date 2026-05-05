package vshnkeycloak

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestCreateIngress(t *testing.T) {
	type args struct {
		comp *vshnv1.VSHNKeycloak
	}

	// Static objects that will always be the same across tests
	keycloakObjectMeta := metav1.ObjectMeta{
		Name: "keycloak",
	}

	ingObjectMeta := metav1.ObjectMeta{
		Annotations: map[string]string{},
		Name:        "keycloak-ingress",
		Namespace:   "vshn-keycloak-keycloak",
	}

	expectedIngressBackend := v1.IngressBackend{
		Service: &v1.IngressServiceBackend{
			Name: "keycloak-keycloakx-http",
			Port: v1.ServiceBackendPort{
				Name: "https",
			},
		},
	}

	tests := []struct {
		name string
		args args
		want *v1.Ingress
	}{
		{
			name: "GivenFQDN_Then_ExpectIngress",
			args: struct {
				comp *vshnv1.VSHNKeycloak
			}{
				comp: &vshnv1.VSHNKeycloak{
					ObjectMeta: keycloakObjectMeta,
					Spec: vshnv1.VSHNKeycloakSpec{
						Parameters: vshnv1.VSHNKeycloakParameters{
							Service: vshnv1.VSHNKeycloakServiceSpec{
								FQDN:         "example.com",
								RelativePath: "/path",
							},
						},
					},
				},
			},
			want: &v1.Ingress{
				ObjectMeta: ingObjectMeta,
				Spec: v1.IngressSpec{
					Rules: []v1.IngressRule{{
						Host: "example.com",
						IngressRuleValue: v1.IngressRuleValue{
							HTTP: &v1.HTTPIngressRuleValue{
								Paths: []v1.HTTPIngressPath{
									{
										Path:     "/path",
										PathType: ptr.To(v1.PathType("Prefix")),
										Backend:  expectedIngressBackend,
									},
								},
							},
						},
					}},
					TLS: []v1.IngressTLS{
						{
							Hosts:      []string{"example.com"},
							SecretName: "keycloak-ingress-cert",
						},
					},
				},
			},
		},
		{
			name: "GivenAppsFQDN_Then_ExpectIngressWithEmptyTLS",
			args: struct {
				comp *vshnv1.VSHNKeycloak
			}{
				comp: &vshnv1.VSHNKeycloak{
					ObjectMeta: keycloakObjectMeta,
					Spec: vshnv1.VSHNKeycloakSpec{
						Parameters: vshnv1.VSHNKeycloakParameters{
							Service: vshnv1.VSHNKeycloakServiceSpec{
								FQDN:         "instance.apps.example.com",
								RelativePath: "/path",
							},
						},
					},
				},
			},
			want: &v1.Ingress{
				ObjectMeta: ingObjectMeta,
				Spec: v1.IngressSpec{
					Rules: []v1.IngressRule{{
						Host: "instance.apps.example.com",
						IngressRuleValue: v1.IngressRuleValue{
							HTTP: &v1.HTTPIngressRuleValue{
								Paths: []v1.HTTPIngressPath{
									{
										Path:     "/path",
										PathType: ptr.To(v1.PathType("Prefix")),
										Backend:  expectedIngressBackend,
									},
								},
							},
						},
					}},
					TLS: []v1.IngressTLS{{}},
				},
			},
		},
		{
			name: "GivenNoFQDN_Then_ExpectNoIngress",
			args: struct {
				comp *vshnv1.VSHNKeycloak
			}{
				comp: &vshnv1.VSHNKeycloak{},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := commontest.LoadRuntimeFromFile(t, "vshnkeycloak/01_default.yaml")
			fqdn := tt.args.comp.Spec.Parameters.Service.FQDN
			relPath := tt.args.comp.Spec.Parameters.Service.RelativePath

			var err error
			var ing *v1.Ingress
			if fqdn != "" { // Handle GivenNoFQDN_Then_ExpectNoIngress
				ing, err = common.GenerateIngress(tt.args.comp, svc, common.IngressConfig{
					FQDNs: []string{fqdn},
					ServiceConfig: common.IngressRuleConfig{
						RelPath:           relPath,
						ServiceNameSuffix: "keycloakx-http",
						ServicePortName:   "https",
					},
					TlsCertBaseName: "keycloak",
				})
			}

			assert.NoError(t, err)

			// By marshalling v1.Ingress first, we can prevent assert.Equal from comparing pointer addresses and thus always failing
			want, _ := json.MarshalIndent(tt.want, "", "")
			got, _ := json.MarshalIndent(ing, "", "")
			assert.Equal(t, string(want), string(got))
		})
	}
}

func TestCreateHTTPRoute(t *testing.T) {
	t.Run("GivenHTTPRouteMode_ExpectHTTPRouteAndListenerSet", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "vshnkeycloak/02_httproute.yaml")

		comp := &vshnv1.VSHNKeycloak{
			ObjectMeta: metav1.ObjectMeta{Name: "keycloak"},
			Spec: vshnv1.VSHNKeycloakSpec{
				Parameters: vshnv1.VSHNKeycloakParameters{
					Service: vshnv1.VSHNKeycloakServiceSpec{
						FQDN:         "example.com",
						RelativePath: "/path",
					},
				},
			},
		}

		result := AddIngress(context.Background(), comp, svc)
		assert.Nil(t, result)

		allDesired := svc.GetAllDesired()
		foundRoute, foundLS, foundGrant, foundBCP := false, false, false, false
		for _, d := range allDesired {
			name := d.Resource.GetName()
			if name == "keycloak-httproute" {
				foundRoute = true
			}
			if name == "keycloak-listenerset" {
				foundLS = true
			}
			if name == "keycloak-httpgrant" {
				foundGrant = true
			}
			if name == "keycloak-backend-tls" {
				foundBCP = true
			}
		}
		assert.True(t, foundRoute, "HTTPRoute must be created")
		assert.True(t, foundLS, "XListenerSet must be created")
		assert.False(t, foundGrant, "ReferenceGrant must NOT be created")
		assert.True(t, foundBCP, "BackendConfigPolicy must be created for TLS backend")

		route := &gatewayv1.HTTPRoute{}
		assert.NoError(t, svc.GetDesiredKubeObject(route, "keycloak-httproute"))
		assert.Len(t, route.Spec.Rules, 1)
		assert.Len(t, route.Spec.Rules[0].Matches, 1)
		path := route.Spec.Rules[0].Matches[0].Path
		assert.Equal(t, gatewayv1.PathMatchPathPrefix, *path.Type)
		assert.Equal(t, "/path", *path.Value, "RelativePath from claim must propagate to HTTPRoute match")
	})

	t.Run("GivenHTTPRouteMode_NoFQDN_ExpectNil", func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "vshnkeycloak/02_httproute.yaml")
		comp := &vshnv1.VSHNKeycloak{}
		result := AddIngress(context.Background(), comp, svc)
		assert.Nil(t, result)
	})
}
