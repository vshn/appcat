package common

import (
	"fmt"
	"strings"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"gopkg.in/yaml.v2"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// IngressConfig contains general information for generating an Ingress obect
type IngressConfig struct {
	AdditionalAnnotations  map[string]string // Optional
	AdditionalIngressNames []string          // Optional
	FQDNs                  []string
	ServiceConfig          IngressRuleConfig
	TlsCertBaseName        string
}

// IngressRuleConfig describes an ingress rule configuration
type IngressRuleConfig struct {
	RelPath           string // Optional, defaults to "/"
	ServiceNameSuffix string // Optional
	ServicePortName   string // Has preference over ServicePortNumber
	ServicePortNumber int32
}

// Checks if an FQDN is part of a reference FQDN, e.g. an OpenShift Apps domain; "*nextcloud*.apps.cluster.com".
// Returns true if yes and FQDN is not a 2nd level subdomain (i.e. *sub2.sub1*.apps.cluster.com)
func IsSingleSubdomainOfRefDomain(fqdn string, reference string) bool {
	if !strings.Contains(fqdn, reference) || reference == "" {
		return false
	}

	noSuffix, _ := strings.CutSuffix(fqdn, reference)
	return len(strings.Split(noSuffix, ".")) == 2 // Handles prefixed dot of reference domain
}

// Obtain ingress annotations and optionally extend them using additionalAnnotations
func getIngressAnnotations(svc *runtime.ServiceRuntime, additionalAnnotations map[string]string) map[string]string {
	annotations := map[string]string{}
	if svc.Config.Data["ingress_annotations"] != "" {
		err := yaml.Unmarshal([]byte(svc.Config.Data["ingress_annotations"]), annotations)
		if err != nil {
			svc.Log.Error(err, "cannot unmarshal ingress annotations from input")
			svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot unmarshal ingress annotations from input: %s", err)))
		}
	} else {
		svc.Log.Info("no ingress annotations are defined")
	}

	for k, v := range additionalAnnotations {
		annotations[k] = v
	}

	return annotations
}

// Creates ingress rules based on a single service name and port. svcNameSuffix is optional and gets appended.
// Will use svcPortName over svcPortNumber (if specified)
func createIngressRule(comp InfoGetter, fqdns []string, ruleConfig IngressRuleConfig) []netv1.IngressRule {
	svcNameSuffix := ruleConfig.ServiceNameSuffix
	if !strings.HasPrefix(svcNameSuffix, "-") && len(svcNameSuffix) > 0 {
		svcNameSuffix = "-" + svcNameSuffix
	}

	relPath := ruleConfig.RelPath
	if relPath == "" {
		relPath = "/"
	}

	ingressRules := []netv1.IngressRule{}

	// prefer ServicePortName over ServicePortNumber
	serviceBackendPort := netv1.ServiceBackendPort{}
	if ruleConfig.ServicePortName != "" {
		serviceBackendPort.Name = ruleConfig.ServicePortName
	} else {
		serviceBackendPort.Number = ruleConfig.ServicePortNumber
	}

	for _, fqdn := range fqdns {
		rule := netv1.IngressRule{
			Host: fqdn,
			IngressRuleValue: netv1.IngressRuleValue{
				HTTP: &netv1.HTTPIngressRuleValue{
					Paths: []netv1.HTTPIngressPath{
						{
							Path:     relPath,
							PathType: ptr.To(netv1.PathType("Prefix")),
							Backend: netv1.IngressBackend{
								Service: &netv1.IngressServiceBackend{
									Name: comp.GetName() + svcNameSuffix,
									Port: serviceBackendPort,
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

// Generate an ingress TLS secret name as "<baseName>-ingress-cert"
func generateTlsSecretName(baseName string) string {
	return strings.Trim(baseName, "-") + "-ingress-cert"
}

// Generate an ingress name. additionalNames will be assembled as "comp.GetName()-<n1>-<n2>-<nX>-ingress"
func generateIngressName(comp InfoGetter, additionalNames ...string) string {
	ingressName := comp.GetName()
	if len(additionalNames) > 0 {
		for _, n := range additionalNames {
			n = strings.Trim(n, "-")
			ingressName = ingressName + "-" + n
		}
	}
	ingressName = ingressName + "-ingress"

	return ingressName
}

// Generate up to 2 ingresses that bundle FQDNs depending on the following:
// FQDNs that are one subdomain ON defaultAppsDomain (e.g. sub1.apps.cluster.com) -> Empty TLS config (uses wildcard cert on OCP).
// FQDNs that do not statisfy the former -> TLS config using a Let's Encrypt certificate.
func GenerateBundledIngresses(comp InfoGetter, svc *runtime.ServiceRuntime, ingressConfig IngressConfig) ([]*netv1.Ingress, error) {
	if len(ingressConfig.FQDNs) == 0 {
		return nil, fmt.Errorf("no FQDNs")
	}

	for _, fqdn := range ingressConfig.FQDNs {
		if fqdn == "" {
			return nil, fmt.Errorf("an empty FQDN has been passed. Passed FQDNs are: %v", ingressConfig.FQDNs)
		}
	}

	// bool: true -> Use wildcard, false -> Use LE, []string: List of FQDNs
	ingressMap := map[bool][]string{}

	ocpDefaultAppsDomain := svc.Config.Data["ocpDefaultAppsDomain"]
	svc.Log.Info(fmt.Sprintf("ocpAppsDomain is: '%s'", ocpDefaultAppsDomain))

	if ocpDefaultAppsDomain != "" {
		for _, fqdn := range ingressConfig.FQDNs {
			useWildcard := IsSingleSubdomainOfRefDomain(fqdn, ocpDefaultAppsDomain)
			svc.Log.Info(fmt.Sprintf("FQDN %s will use wildcard cert: %v", fqdn, useWildcard))
			ingressMap[useWildcard] = append(ingressMap[useWildcard], fqdn)
		}
	} else {
		ingressMap[false] = ingressConfig.FQDNs
	}

	// Create ingresses that bundle FQDNs depending on their certificate requirements (LE/Wildcard)
	var ingresses []*netv1.Ingress
	fqdnsLetsEncrypt, fqdnsWildcard := ingressMap[false], ingressMap[true]

	tlsName := generateTlsSecretName(ingressConfig.TlsCertBaseName)
	if tlsName == "" {
		return nil, fmt.Errorf("a TLS cert base name must be defined")
	}

	ingressMetadata := metav1.ObjectMeta{
		Namespace:   comp.GetInstanceNamespace(),
		Annotations: getIngressAnnotations(svc, ingressConfig.AdditionalAnnotations),
	}

	// Ingress using Let's Encrypt
	if len(fqdnsLetsEncrypt) > 0 {
		ingressMetadata.Name = generateIngressName(comp, "letsencrypt")
		ingresses = append(ingresses, &netv1.Ingress{
			ObjectMeta: ingressMetadata,
			Spec: netv1.IngressSpec{
				Rules: createIngressRule(comp, fqdnsLetsEncrypt, ingressConfig.ServiceConfig),
				TLS: []netv1.IngressTLS{
					{
						Hosts:      fqdnsLetsEncrypt,
						SecretName: tlsName,
					},
				},
			},
		})
	}

	// Ingress using apps domain wildcard
	if len(fqdnsWildcard) > 0 {
		ingressMetadata.Name = generateIngressName(comp, "wildcard")
		ingresses = append(ingresses, &netv1.Ingress{
			ObjectMeta: ingressMetadata,
			Spec: netv1.IngressSpec{
				Rules: createIngressRule(comp, fqdnsWildcard, ingressConfig.ServiceConfig),
				TLS:   []netv1.IngressTLS{{}},
			},
		})
	}

	return ingresses, nil
}

// Generate an Ingress containing a single FQDN using a TLS config as such:
// FQDN is one subdomain ON defaultAppsDomain (e.g. sub1.apps.cluster.com) -> Empty TLS config (uses wildcard cert on OCP).
// FQDN does not statisfy the former -> TLS config using a Let's Encrypt certificate.
func GenerateIngress(comp InfoGetter, svc *runtime.ServiceRuntime, ingressConfig IngressConfig) (*netv1.Ingress, error) {
	if len(ingressConfig.FQDNs) == 0 || ingressConfig.FQDNs[0] == "" {
		return nil, fmt.Errorf("no FQDN passed")
	}

	fqdn := ingressConfig.FQDNs[0]
	if len(ingressConfig.FQDNs) > 1 {
		svc.AddResult(
			runtime.NewWarningResult(
				fmt.Sprintf("More than 1 FQDN has been passed to a singleton ingress object, using first available: %s", fqdn),
			),
		)
	}

	annotations := getIngressAnnotations(svc, ingressConfig.AdditionalAnnotations)

	tlsName := ingressConfig.TlsCertBaseName
	if tlsName == "" {
		return nil, fmt.Errorf("a TLS cert base name must be defined")
	}
	for _, an := range ingressConfig.AdditionalIngressNames {
		tlsName = tlsName + "-" + strings.Trim(an, "-")
	}

	tlsConfig := netv1.IngressTLS{}
	if !IsSingleSubdomainOfRefDomain(fqdn, svc.Config.Data["ocpDefaultAppsDomain"]) {
		tlsConfig.Hosts = []string{fqdn}
		tlsConfig.SecretName = generateTlsSecretName(tlsName)
	}

	ingress := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        generateIngressName(comp, ingressConfig.AdditionalIngressNames...),
			Namespace:   comp.GetInstanceNamespace(),
			Annotations: annotations,
		},
		Spec: netv1.IngressSpec{
			Rules: createIngressRule(comp, []string{fqdn}, ingressConfig.ServiceConfig),
			TLS:   []netv1.IngressTLS{tlsConfig},
		},
	}

	return ingress, nil
}

// Apply generated ingresses using svc.SetDesiredKubeObject()
func CreateIngresses(comp InfoGetter, svc *runtime.ServiceRuntime, ingresses []*netv1.Ingress, opts ...runtime.KubeObjectOption) error {
	for _, ingress := range ingresses {
		err := svc.SetDesiredKubeObject(ingress, ingress.Name, opts...)
		if err != nil {
			return err
		}
	}

	return nil
}
