package common

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/tcproute"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GateExternalCertReadiness keeps the <name>-server-cert (and thus the
// composite) unready until the external endpoint (gateway domain or
// loadbalancer IP) is present in the connection details and covered by the
// server certificate's SANs. This forces Crossplane to keep reconciling so the
// external connection details get published promptly. serviceType is the
// instance's network.serviceType; secretName is the issued server cert secret.
func GateExternalCertReadiness(svc *runtime.ServiceRuntime, comp InfoGetter, serviceType, secretName, gatewayHost, loadbalancerIP string) error {
	if !ExternalAccessEnabled(svc) {
		return nil
	}

	certResource := comp.GetName() + "-server-cert"
	observer := certResource + "-tls-observer"

	switch serviceType {
	case tcproute.ServiceTypeTCPGateway:
		if gatewayHost == "" {
			svc.SetDesiredResourceReadiness(certResource, runtime.ResourceUnReady)
			return nil
		}
		return WaitForCertSAN(svc, certResource, observer, secretName, comp.GetInstanceNamespace(), CertHasDNS(gatewayHost))
	case string(corev1.ServiceTypeLoadBalancer):
		if loadbalancerIP == "" {
			svc.SetDesiredResourceReadiness(certResource, runtime.ResourceUnReady)
			return nil
		}
		return WaitForCertSAN(svc, certResource, observer, secretName, comp.GetInstanceNamespace(), CertHasIP(loadbalancerIP))
	}

	return nil
}

// CertSANMatcher checks whether a parsed x509 certificate contains the
// expected SAN (Subject Alternative Name).
type CertSANMatcher func(cert *x509.Certificate) bool

// CertHasIP returns a matcher that checks if the certificate contains the given IP address.
func CertHasIP(ip string) CertSANMatcher {
	return func(cert *x509.Certificate) bool {
		for _, certIP := range cert.IPAddresses {
			if certIP.String() == ip {
				return true
			}
		}
		return false
	}
}

// CertHasDNS returns a matcher that checks if the certificate contains the given DNS name.
func CertHasDNS(dnsName string) CertSANMatcher {
	return func(cert *x509.Certificate) bool {
		for _, name := range cert.DNSNames {
			if name == dnsName {
				return true
			}
		}
		return false
	}
}

// waitForCertSAN marks the certificate resource as unready until the observed
// TLS certificate contains the expected SAN. Once the SAN is present the
// resource is marked ready again.
//
// This prevents Crossplane from reporting the instance as ready before the
// certificate covers the external endpoint (LoadBalancer IP or gateway domain).
func WaitForCertSAN(svc *runtime.ServiceRuntime, certResourceName, observerName, secretName, instanceNamespace string, match CertSANMatcher) error {
	svc.SetDesiredResourceReadiness(certResourceName, runtime.ResourceUnReady)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: instanceNamespace,
		},
	}

	if err := svc.SetDesiredKubeObject(secret, observerName, runtime.KubeOptionObserve, runtime.KubeOptionAllowDeletion); err != nil {
		svc.Log.Error(err, "cannot deploy certificate secret observer")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot deploy certificate secret observer: %s", err)))
	}

	obsSecret := &corev1.Secret{}
	if err := svc.GetObservedKubeObject(obsSecret, observerName); err != nil {
		svc.Log.Info("certificate secret not yet observed")
		return nil
	}

	block, _ := pem.Decode(obsSecret.Data["tls.crt"])
	if block == nil {
		svc.Log.Info("cannot decode tls certificate")
		return nil
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		svc.Log.Error(err, "cannot parse tls certificate")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot parse tls certificate: %s", err)))
		return nil
	}

	if match(cert) {
		svc.SetDesiredResourceReadiness(certResourceName, runtime.ResourceReady)
	}

	return nil
}
