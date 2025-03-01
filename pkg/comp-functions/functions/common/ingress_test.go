package common

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	netv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	compName           = "my-service"
	tlsCertBaseName    = "my-service"
	svcBackendPortName = "https"
	svcNameSuffix      = "http"
	relativePath       = "/path"
)

// Static objects that will always be the same across tests
var ingObjectMeta metav1.ObjectMeta = metav1.ObjectMeta{
	Annotations: map[string]string{},
	Name:        compName + "-ingress", // May change in tests
	Namespace:   "vshn-" + compName,
}

var svcName string = compName + "-" + svcNameSuffix
var expectedIngressBackend v1.IngressBackend = v1.IngressBackend{
	Service: &v1.IngressServiceBackend{
		Name: svcName,
		Port: v1.ServiceBackendPort{
			Name: svcBackendPortName,
		},
	},
}

// Barebones dummy composite for testing
type baaSComposite struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	InfoGetter
	Spec baaSSpec
}

type baaSSpec struct {
	Parameters baaSParameters
}

type baaSParameters struct {
	Service baaSServiceSpec
}

type baaSServiceSpec struct {
	FQDN         string
	FQDNs        []string
	RelativePath string
}

func (b *baaSComposite) GetLabels() map[string]string {
	return b.Labels
}
func (b *baaSComposite) GetName() string {
	return b.ObjectMeta.Name
}
func (b *baaSComposite) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-%s", b.GetName())
}

type args struct {
	comp *baaSComposite
}

// Test the generation of multiple ingress objects with various configurations
func TestGenerateMultipleIngresses(t *testing.T) {
	ingObjectMetaLetsEncrypt := ingObjectMeta
	ingObjectMetaLetsEncrypt.Name = compName + "-letsencrypt-ingress"
	ingObjectMetaWildcard := ingObjectMeta
	ingObjectMetaWildcard.Name = compName + "-wildcard-ingress"

	test := struct {
		name string
		args args
		want []*v1.Ingress
	}{
		name: "ExpectMultipleIngresses_WithSeparatedFQDNs",
		args: struct {
			comp *baaSComposite
		}{
			comp: &baaSComposite{
				ObjectMeta: metav1.ObjectMeta{
					Name: compName,
				},
				Spec: baaSSpec{
					Parameters: baaSParameters{
						Service: baaSServiceSpec{
							FQDNs:        []string{"my-domain.com", "instance.apps.example.com"},
							RelativePath: relativePath,
						},
					},
				},
			},
		},
		want: []*v1.Ingress{
			// Let's encrypt ingress
			{
				ObjectMeta: ingObjectMetaLetsEncrypt,
				Spec: v1.IngressSpec{
					Rules: []v1.IngressRule{{
						Host: "my-domain.com",
						IngressRuleValue: v1.IngressRuleValue{
							HTTP: &v1.HTTPIngressRuleValue{
								Paths: []v1.HTTPIngressPath{
									{
										Path:     relativePath,
										PathType: ptr.To(v1.PathType("Prefix")),
										Backend:  expectedIngressBackend,
									},
								},
							},
						},
					}},
					TLS: []v1.IngressTLS{
						{
							Hosts:      []string{"my-domain.com"},
							SecretName: tlsCertBaseName + "-ingress-cert",
						},
					},
				},
			},
			// Wildcard ingress
			{
				ObjectMeta: ingObjectMetaWildcard,
				Spec: v1.IngressSpec{
					Rules: []v1.IngressRule{{
						Host: "instance.apps.example.com",
						IngressRuleValue: v1.IngressRuleValue{
							HTTP: &v1.HTTPIngressRuleValue{
								Paths: []v1.HTTPIngressPath{
									{
										Path:     relativePath,
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
	}

	t.Run(test.name, func(t *testing.T) {
		svc := commontest.LoadRuntimeFromFile(t, "common/02_ingress.yaml")
		fqdns := test.args.comp.Spec.Parameters.Service.FQDNs
		relPath := test.args.comp.Spec.Parameters.Service.RelativePath

		ings, err := GenerateBundledIngresses(test.args.comp, svc, IngressConfig{
			FQDNs: fqdns,
			ServiceConfig: IngressRuleConfig{
				RelPath:           relPath,
				ServiceNameSuffix: svcNameSuffix,
				ServicePortName:   svcBackendPortName,
			},
			TlsCertBaseName: tlsCertBaseName,
		})

		assert.NoError(t, err)
		for i, _ := range ings {
			fmt.Printf("Testing ingress: %s\n", ings[i].Name)
			doMarshalledAssert(t, test.want[i], ings[i])
		}
	})
}

// Test the generation of a single ingress object with various configurations
func TestGenerateSingleIngress(t *testing.T) {
	tests := []struct {
		name string
		args args
		want *v1.Ingress
	}{
		{
			name: "GivenNonAppsFQDN_Then_ExpectIngress_WithPopulatedTLS",
			args: struct {
				comp *baaSComposite
			}{
				comp: &baaSComposite{
					ObjectMeta: metav1.ObjectMeta{
						Name: compName,
					},
					Spec: baaSSpec{
						Parameters: baaSParameters{
							Service: baaSServiceSpec{
								FQDN:         "my-domain.com",
								RelativePath: relativePath,
							},
						},
					},
				},
			},
			want: &v1.Ingress{
				ObjectMeta: ingObjectMeta,
				Spec: v1.IngressSpec{
					Rules: []v1.IngressRule{{
						Host: "my-domain.com",
						IngressRuleValue: v1.IngressRuleValue{
							HTTP: &v1.HTTPIngressRuleValue{
								Paths: []v1.HTTPIngressPath{
									{
										Path:     relativePath,
										PathType: ptr.To(v1.PathType("Prefix")),
										Backend:  expectedIngressBackend,
									},
								},
							},
						},
					}},
					TLS: []v1.IngressTLS{
						{
							Hosts:      []string{"my-domain.com"},
							SecretName: tlsCertBaseName + "-ingress-cert",
						},
					},
				},
			},
		},
		{
			name: "GivenAppsFQDN_Then_ExpectIngressWithEmptyTLS",
			args: struct {
				comp *baaSComposite
			}{
				comp: &baaSComposite{
					ObjectMeta: metav1.ObjectMeta{
						Name: compName,
					},
					Spec: baaSSpec{
						Parameters: baaSParameters{
							Service: baaSServiceSpec{
								FQDN:         "instance.apps.example.com",
								RelativePath: relativePath,
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
										Path:     relativePath,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := commontest.LoadRuntimeFromFile(t, "common/02_ingress.yaml")
			fqdn := tt.args.comp.Spec.Parameters.Service.FQDN
			relPath := tt.args.comp.Spec.Parameters.Service.RelativePath

			ing, err := GenerateIngress(tt.args.comp, svc, IngressConfig{
				FQDNs: []string{fqdn},
				ServiceConfig: IngressRuleConfig{
					RelPath:           relPath,
					ServiceNameSuffix: svcNameSuffix,
					ServicePortName:   svcBackendPortName,
				},
				TlsCertBaseName: tlsCertBaseName,
			})

			assert.NoError(t, err)
			doMarshalledAssert(t, tt.want, ing)
		})
	}
}

func TestIngressRuleGeneration(t *testing.T) {
	comp := &baaSComposite{
		ObjectMeta: metav1.ObjectMeta{
			Name: compName,
		},
		Spec: baaSSpec{
			Parameters: baaSParameters{
				Service: baaSServiceSpec{
					FQDNs: []string{
						"instance.apps.example.com",
						"another.instance.apps.example.com",
						"epic-instance.my-domain.com",
					},
					RelativePath: relativePath,
				},
			},
		},
	}
	fqdns := comp.Spec.Parameters.Service.FQDNs

	t.Run("GivenFQDNs_ExpectRules", func(t *testing.T) {
		rules, err := createIngressRule(comp, comp.Spec.Parameters.Service.FQDNs, IngressRuleConfig{
			RelPath:           relativePath,
			ServiceNameSuffix: svcNameSuffix,
			ServicePortName:   svcBackendPortName,
			ServicePortNumber: 1337,
		})

		assert.NoError(t, err)
		assert.Len(t, rules, len(fqdns))

		for i, r := range rules {
			nameWithSuffix := comp.GetName() + "-" + svcNameSuffix
			assert.Equal(t, nameWithSuffix, r.HTTP.Paths[0].Backend.Service.Name)
			assert.Equal(t, relativePath, r.HTTP.Paths[0].Path)
			assert.Equal(t, netv1.PathType("Prefix"), *r.HTTP.Paths[0].PathType)
			assert.Equal(t, nameWithSuffix, r.HTTP.Paths[0].Backend.Service.Name)
			assert.Equal(t, svcBackendPortName, r.HTTP.Paths[0].Backend.Service.Port.Name)
			assert.Equal(t, fqdns[i], r.Host)
		}
	})

	t.Run("GivenNoRelativePath_ExpectDefault", func(t *testing.T) {
		comp.Spec.Parameters.Service.RelativePath = ""

		rules, err := createIngressRule(comp, comp.Spec.Parameters.Service.FQDNs, IngressRuleConfig{
			ServicePortName: svcBackendPortName,
		})

		assert.NoError(t, err)
		assert.Equal(t, "/", rules[0].HTTP.Paths[0].Path)
	})

	t.Run("GivenNeitherPortNameNorPortNumber_ExpectError", func(t *testing.T) {
		_, err := createIngressRule(comp, []string{"example.com"}, IngressRuleConfig{})
		assert.Error(t, err)
	})

	t.Run("GivenNoFQDNs_ExpectError", func(t *testing.T) {
		_, err := createIngressRule(comp, []string{}, IngressRuleConfig{})
		assert.Error(t, err)
	})

	t.Run("GivenSvcPortNameAndPortNumber_ExpectPortNameToBeUsed", func(t *testing.T) {
		rules, err := createIngressRule(comp, []string{"example.com"}, IngressRuleConfig{
			ServicePortName:   svcBackendPortName,
			ServicePortNumber: 1337,
		})

		assert.NoError(t, err)
		for _, r := range rules {
			assert.Equal(t, svcBackendPortName, r.HTTP.Paths[0].Backend.Service.Port.Name)
			assert.Equal(t, int32(0), r.HTTP.Paths[0].Backend.Service.Port.Number)
		}
	})
}

func TestIngressNameGeneration(t *testing.T) {
	t.Run("GivenAdditionalNames_ExpectIngressSuffix", func(t *testing.T) {
		comp := &baaSComposite{
			ObjectMeta: metav1.ObjectMeta{
				Name: compName,
			},
		}

		name := generateIngressName(comp, "a", "b", "c")
		assert.Equal(t, comp.GetName()+"-a-b-c-ingress", name)
	})
}

func doMarshalledAssert(t *testing.T, want any, got any) {
	// By marshalling v1.Ingress first, we can prevent assert.Equal from comparing pointer addresses and thus always failing
	wantMarshal, _ := json.MarshalIndent(want, "", "")
	gotMarshal, _ := json.MarshalIndent(got, "", "")
	assert.Equal(t, string(wantMarshal), string(gotMarshal))
}
