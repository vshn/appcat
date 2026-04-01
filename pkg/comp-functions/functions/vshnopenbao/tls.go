package vshnopenbao

import (
	"context"
	"fmt"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultCertificateDuration    = 87600 * time.Hour
	defaultCertificateRenewBefore = 2400 * time.Hour
)

func setupTLSCertificates(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) *xfnproto.Result {
	serviceName := comp.GetName()
	ns := comp.GetInstanceNamespace()
	details := getServiceDetails(serviceName)

	selfSignedIssuerOpts := &common.TLSOptions{
		IssuerOptions: []common.IssuerOption{
			withIssuerOfTypeSelfSigned(),
		},
	}
	selfSignedIssuer := createIssuer(ns, details.SelfSignedIssuerName, selfSignedIssuerOpts)
	err := svc.SetDesiredKubeObject(selfSignedIssuer, details.SelfSignedIssuerName, selfSignedIssuerOpts.KubeOptions...)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create Self Signed Issuer for OpenBao %w", err).Error())
	}

	rootCAOpts := &common.TLSOptions{
		CertOptions: []common.CertOptions{
			withCertificateSecretName(details.RootCASecretName),
			withCertificateDuration(defaultCertificateDuration),
			withCertificateRenewBefore(defaultCertificateRenewBefore),
			withCertificateIssuerRef(details.SelfSignedIssuerName),
			withCertificateIsCA(true),
		},
		KubeOptions: []runtime.KubeObjectOption{},
	}
	rootCA := createCertificate(ns, details.RootCAName, rootCAOpts)
	err = svc.SetDesiredKubeObject(rootCA, details.RootCAName, rootCAOpts.KubeOptions...)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create Root CA with Self Signed Issuer for OpenBao %w", err).Error())
	}

	rootCAIssuerOpts := &common.TLSOptions{
		IssuerOptions: []common.IssuerOption{
			withIssuerOfTypeCA(details.RootCASecretName),
		},
	}
	rootCAIssuer := createIssuer(ns, details.RootCAIssuerName, rootCAIssuerOpts)
	err = svc.SetDesiredKubeObject(rootCAIssuer, details.RootCAIssuerName, rootCAIssuerOpts.KubeOptions...)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create Root CA Issuer for OpenBao %w", err).Error())
	}

	openBaoServerCertOpts := &common.TLSOptions{
		CertOptions: []common.CertOptions{
			withCertificateSecretName(details.ServerCertSecretName),
			withCertificateDuration(defaultCertificateDuration),
			withCertificateRenewBefore(defaultCertificateRenewBefore),
			withCertificateIssuerRef(details.RootCAIssuerName),
			withCertificateDNSName(serviceName + "." + ns + ".svc.cluster.local"),
			withCertificateDNSName(serviceName + "." + ns + ".svc"),
			withCertificateUsages([]cmv1.KeyUsage{"server auth", "client auth"}),
			withCertificateIsCA(false),
		},
		KubeOptions: []runtime.KubeObjectOption{
			runtime.KubeOptionAddConnectionDetails(ns,
				xkube.ConnectionDetail{
					ObjectReference: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  ns,
						Name:       details.ServerCertSecretName,
						FieldPath:  "data[ca.crt]",
					},
					ToConnectionSecretKey: "ca.crt",
				},
				xkube.ConnectionDetail{
					ObjectReference: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  ns,
						Name:       details.ServerCertSecretName,
						FieldPath:  "data[tls.crt]",
					},
					ToConnectionSecretKey: "tls.crt",
				},
				xkube.ConnectionDetail{
					ObjectReference: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  ns,
						Name:       details.ServerCertSecretName,
						FieldPath:  "data[tls.key]",
					},
					ToConnectionSecretKey: "tls.key",
				},
			),
		},
	}
	openBaoServerCert := createCertificate(ns, details.ServerCertName, openBaoServerCertOpts)
	err = svc.SetDesiredKubeObject(openBaoServerCert, details.ServerCertName, openBaoServerCertOpts.KubeOptions...)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot create Server Certificate for OpenBao %w", err).Error())
	}

	return nil
}

func createIssuer(namespace string, issuerName string, opts *common.TLSOptions) *cmv1.Issuer {
	resource := &cmv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      issuerName,
			Namespace: namespace,
		},
		Spec: cmv1.IssuerSpec{
			IssuerConfig: cmv1.IssuerConfig{},
		},
	}

	if opts.IssuerOptions != nil {
		for _, opt := range opts.IssuerOptions {
			opt(resource)
		}
	}

	return resource
}

func createCertificate(namespace string, certName string, opts *common.TLSOptions) *cmv1.Certificate {
	resource := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certName,
			Namespace: namespace,
		},
		Spec: cmv1.CertificateSpec{
			PrivateKey: &cmv1.CertificatePrivateKey{
				Algorithm: cmv1.RSAKeyAlgorithm,
				Encoding:  cmv1.PKCS1,
				Size:      4096,
			},
			CommonName: certName + "-ca",
			Subject: &cmv1.X509Subject{
				Organizations: []string{
					"vshn-appcat-ca",
				},
			},
		},
	}

	if opts.CertOptions != nil {
		for _, opt := range opts.CertOptions {
			opt(resource)
		}
	}

	return resource
}

func withIssuerOfTypeCA(secretName string) common.IssuerOption {
	return func(issuer *cmv1.Issuer) {
		issuer.Spec.CA = &cmv1.CAIssuer{
			SecretName: secretName,
		}
	}
}

func withIssuerOfTypeSelfSigned() common.IssuerOption {
	return func(issuer *cmv1.Issuer) {
		issuer.Spec.SelfSigned = &cmv1.SelfSignedIssuer{
			CRLDistributionPoints: []string{},
		}
	}
}

func withCertificateSecretName(secretName string) common.CertOptions {
	return func(cert *cmv1.Certificate) {
		cert.Spec.SecretName = secretName
	}
}

func withCertificateDNSName(dnsName string) common.CertOptions {
	return func(cert *cmv1.Certificate) {
		if cert.Spec.DNSNames == nil {
			cert.Spec.DNSNames = []string{}
		}
		cert.Spec.DNSNames = append(cert.Spec.DNSNames, dnsName)
	}
}

func withCertificateIsCA(isCA bool) common.CertOptions {
	return func(cert *cmv1.Certificate) {
		cert.Spec.IsCA = isCA
	}
}

func withCertificateIssuerRef(issuerName string) common.CertOptions {
	return func(cert *cmv1.Certificate) {
		cert.Spec.IssuerRef = certmgrv1.ObjectReference{
			Name:  issuerName,
			Kind:  "Issuer",
			Group: "cert-manager.io",
		}
	}
}

func withCertificateDuration(time time.Duration) common.CertOptions {
	return func(cert *cmv1.Certificate) {
		cert.Spec.Duration = &metav1.Duration{
			Duration: time,
		}
	}
}

func withCertificateRenewBefore(time time.Duration) common.CertOptions {
	return func(cert *cmv1.Certificate) {
		cert.Spec.RenewBefore = &metav1.Duration{
			Duration: time,
		}
	}
}

func withCertificateUsages(usages []cmv1.KeyUsage) common.CertOptions {
	return func(cert *cmv1.Certificate) {
		cert.Spec.Usages = usages
	}
}
