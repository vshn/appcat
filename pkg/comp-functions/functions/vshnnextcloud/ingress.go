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

	fqdns := comp.Spec.Parameters.Service.FQDN

	// bool: true > Use wildcard, false > Use LE, []string: List of FQDNs
	ingressMap := map[bool][]string{}
	ocpDefaultAppsDomain := svc.Config.Data["ocpDefaultAppsDomain"]
	if ocpDefaultAppsDomain != "" {
		for _, fqdn := range fqdns {
			useWildcard := false
			split := strings.Split(fqdn, ocpDefaultAppsDomain)
			if len(split) >= 2 { // FQDN is part of ocpDefaultAppsDomain
				useWildcard = strings.Count(split[0], ".") <= 1
			}

			ingressMap[useWildcard] = append(ingressMap[useWildcard], fqdn)
		}
	} else {
		ingressMap[false] = fqdns
	}

	// Create ingresses that bundle FQDNs depending on their certificate requirements (LE/Wildcard)
	fqdnsLetsEncrypt, fqdnsWildcard := ingressMap[false], ingressMap[true]
	ingressBaseMeta := metav1.ObjectMeta{
		Name:        comp.GetName(),
		Namespace:   comp.GetInstanceNamespace(),
		Annotations: annotations,
	}
	var ingresses []*netv1.Ingress

	if len(fqdnsLetsEncrypt) > 0 {
		ingresses = append(ingresses, &netv1.Ingress{
			ObjectMeta: ingressBaseMeta,
			Spec: netv1.IngressSpec{
				Rules: createIngressRule(comp, fqdnsLetsEncrypt),
				TLS: []netv1.IngressTLS{
					{
						Hosts:      fqdns,
						SecretName: "nextcloud-ingress-cert",
					},
				},
			},
		})
	}

	// Ingress using apps domain wildcard
	if len(fqdnsWildcard) > 0 {
		ingresses = append(ingresses, &netv1.Ingress{
			ObjectMeta: ingressBaseMeta,
			Spec: netv1.IngressSpec{
				Rules: createIngressRule(comp, fqdnsWildcard),
				TLS:   []netv1.IngressTLS{},
			},
		})
	}

	for _, ingress := range ingresses {
		namePrefix := "-ingress-le"
		if len(ingress.Spec.TLS) == 0 {
			namePrefix = "-ingress-wildcard"
		}
		err = svc.SetDesiredKubeObject(ingress, comp.GetName()+namePrefix)
		if err != nil {
			return runtime.NewWarningResult(fmt.Sprintf("cannot set create ingress (%s): %s", namePrefix, err))
		}
	}

	return nil
}

func createIngressRule(comp *vshnv1.VSHNNextcloud, fqdns []string) []netv1.IngressRule {
	ingressRules := []netv1.IngressRule{}

	for _, fqdn := range fqdns {
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
