package vshnpostgrescnpg

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// certSANMatcher checks whether a parsed x509 certificate contains the
// expected SAN (Subject Alternative Name). Returns true if the cert is valid.
type certSANMatcher func(cert *x509.Certificate) bool

// certHasIP returns a matcher that checks if the certificate contains the given IP address.
func certHasIP(ip string) certSANMatcher {
	return func(cert *x509.Certificate) bool {
		for _, certIP := range cert.IPAddresses {
			if certIP.String() == ip {
				return true
			}
		}
		return false
	}
}

// certHasDNS returns a matcher that checks if the certificate contains the given DNS name.
func certHasDNS(dnsName string) certSANMatcher {
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
func waitForCertSAN(svc *runtime.ServiceRuntime, compName, instanceNamespace string, match certSANMatcher) error {
	svc.SetDesiredResourceReadiness("certificate", runtime.ResourceUnReady)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certificateSecretName,
			Namespace: instanceNamespace,
		},
	}

	err := svc.SetDesiredKubeObject(secret, compName+"-tls-observer", runtime.KubeOptionObserve, runtime.KubeOptionAllowDeletion)
	if err != nil {
		svc.Log.Error(err, "cannot deploy certificate secret observer")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot deploy certificate secret observer: %s", err)))
	}

	obsSecret := &corev1.Secret{}

	err = svc.GetObservedKubeObject(obsSecret, compName+"-tls-observer")
	if err != nil {
		svc.Log.Error(err, "cannot observe certificate secret")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot observe certificate secret: %s", err)))
	}

	block, _ := pem.Decode(obsSecret.Data["tls.crt"])
	if block == nil {
		svc.Log.Info("cannot decode tls certificate")
		svc.AddResult(runtime.NewWarningResult("cannot decode tls certificate"))
		return nil
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		svc.Log.Error(err, "cannot parse tls certificate")
		svc.AddResult(runtime.NewWarningResult(fmt.Sprintf("cannot parse tls certificate: %s", err)))
	}

	if cert == nil {
		err := fmt.Errorf("certificate is nil, please check issues with cert-manager")
		svc.AddResult(runtime.NewFatalResult(err))
		return err
	}

	if match(cert) {
		svc.SetDesiredResourceReadiness("certificate", runtime.ResourceReady)
	}

	return nil
}
