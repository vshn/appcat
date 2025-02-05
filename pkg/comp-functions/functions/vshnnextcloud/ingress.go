package vshnnextcloud

import (
	"context"
	"errors"
	"fmt"
	"strings"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"gopkg.in/yaml.v2"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// AddIngress adds an inrgess to the Nextcloud instance.
func AddIngress(_ context.Context, comp *vshnv1.VSHNNextcloud, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	if len(comp.Spec.Parameters.Service.FQDN) == 0 {
		return runtime.NewFatalResult(fmt.Errorf("FQDN array is empty, but requires at least one entry, %w", errors.New("empty fqdn")))
	}

	annotations := map[string]string{}
	if svc.Config.Data["ingress_annotations"] != "" {
		err := yaml.Unmarshal([]byte(svc.Config.Data["ingress_annotations"]), annotations)
		if err != nil {
			svc.Log.Error(err, "cannot unmarshal ingress annotations from input")
			svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot unmarshal ingress annotations from input: %s", err)))
		}
	}

	var ocpDefaultAppsDomain string
	if svc.Config.Data["ocpDefaultAppsDomain"] != "" {
		err := yaml.Unmarshal([]byte(svc.Config.Data["ocpDefaultAppsDomain"]), ocpDefaultAppsDomain)
		if err != nil {
			svc.Log.Error(err, "cannot unmarshal ocpDefaultAppsDomain from input")
			svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot unmarshal ocpDefaultAppsDomain from input: %s", err)))
		}
	}

	usesDefaultAppsDomain := false
	if ocpDefaultAppsDomain != "" {
		for _, fqdn := range comp.Spec.Parameters.Service.FQDN {
			usesDefaultAppsDomain = strings.Contains(fqdn, ocpDefaultAppsDomain)
			if usesDefaultAppsDomain {
				break
			}
		}
	}

	var ingressTls []netv1.IngressTLS
	if usesDefaultAppsDomain {
		ingressTls = append(ingressTls, netv1.IngressTLS{})
	} else {
		ingressTls = append(ingressTls, netv1.IngressTLS{
			Hosts:      comp.Spec.Parameters.Service.FQDN,
			SecretName: "nextcloud-ingress-cert",
		})
	}

	ingress := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        comp.GetName(),
			Namespace:   comp.GetInstanceNamespace(),
			Annotations: annotations,
		},
		Spec: netv1.IngressSpec{
			Rules: createIngressRule(comp),
			TLS:   ingressTls,
		},
	}

	err = svc.SetDesiredKubeObject(ingress, comp.GetName()+"-ingress")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot set create ingress: %s", err))
	}

	return nil
}

func createIngressRule(comp *vshnv1.VSHNNextcloud) []netv1.IngressRule {

	ingressRules := []netv1.IngressRule{}

	for _, fqdn := range comp.Spec.Parameters.Service.FQDN {
		rule := netv1.IngressRule{
			Host: fqdn,
			IngressRuleValue: netv1.IngressRuleValue{
				HTTP: &netv1.HTTPIngressRuleValue{
					Paths: []netv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: ptr.To(netv1.PathType("Prefix")),
							Backend: netv1.IngressBackend{
								Service: &netv1.IngressServiceBackend{
									Name: comp.GetName(),
									Port: netv1.ServiceBackendPort{
										Number: 8080,
									},
								},
							},
						},
					},
				},
			},
		}
		ingressRules = append(ingressRules, rule)
	}
	return ingressRules
}
