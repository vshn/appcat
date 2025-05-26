package common

import (
	"context"
	"fmt"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TLSOptions let's you pass advanced configurations to the underlying objects.
type TLSOptions struct {
	// AdditionalSans is a list of additional SANs that should get added to the
	// certificate
	AdditionalSans []string
	// IssuerOptions is a list of additional functions that manipulate the Issuer resource
	IssuerOptions []IssuerOption
	// CertOptions is a list of additional functions that manipulate the Certificate resource
	CertOptions []CertOptions
	//KubeOptions is a list of KubecObjectOptions that will be passed to the SetDesiredKubeObject function
	KubeOptions []runtime.KubeObjectOption
}

type IssuerOption func(*cmv1.Issuer)

type CertOptions func(*cmv1.Certificate)

// CreateTLSCerts creates ssl/tls certificates. Servicename will be concatenated with the given namespace to generate a proper k8s fqdn.
// In addition to an error it also returns the name of the secret containing the server certifcates.
func CreateTLSCerts(ctx context.Context, ns string, serviceName string, svc *runtime.ServiceRuntime, opts *TLSOptions) (string, error) {

	kubeOpts := []runtime.KubeObjectOption{}
	if opts != nil {
		kubeOpts = opts.KubeOptions
	}

	selfSignedIssuer := &cmv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "-selfsigned",
			Namespace: ns,
		},
		Spec: cmv1.IssuerSpec{
			IssuerConfig: cmv1.IssuerConfig{
				SelfSigned: &cmv1.SelfSignedIssuer{
					CRLDistributionPoints: []string{},
				},
			},
		},
	}

	if opts != nil {
		for _, opt := range opts.IssuerOptions {
			opt(selfSignedIssuer)
		}
	}

	err := svc.SetDesiredKubeObject(selfSignedIssuer, serviceName+"-selfsigned-issuer", kubeOpts...)
	if err != nil {
		err = fmt.Errorf("cannot create selfSignedIssuer object: %w", err)
		return "", err
	}

	serverCertsSecret := "tls-server-certificate"

	caCert := &cmv1.Certificate{

		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "-ca",
			Namespace: ns,
		},
		Spec: cmv1.CertificateSpec{
			SecretName: "tls-ca-certificate",
			Duration: &metav1.Duration{
				Duration: time.Duration(87600 * time.Hour),
			},
			RenewBefore: &metav1.Duration{
				Duration: time.Duration(2400 * time.Hour),
			},
			Subject: &cmv1.X509Subject{
				Organizations: []string{
					"vshn-appcat-ca",
				},
			},
			IsCA: true,
			PrivateKey: &cmv1.CertificatePrivateKey{
				Algorithm: cmv1.ECDSAKeyAlgorithm,
				Encoding:  cmv1.PKCS1,
				Size:      256,
			},
			CommonName: serviceName + "-ca",
			IssuerRef: certmgrv1.ObjectReference{
				Name:  serviceName + "-selfsigned",
				Kind:  "Issuer",
				Group: "cert-manager.io",
			},
		},
	}

	if opts != nil {
		for _, opt := range opts.CertOptions {
			opt(caCert)
		}
	}

	err = svc.SetDesiredKubeObject(caCert, serviceName+"-ca-cert", kubeOpts...)
	if err != nil {
		err = fmt.Errorf("cannot create caCert object: %w", err)
		return serverCertsSecret, err
	}

	caIssuer := &cmv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "-ca",
			Namespace: ns,
		},
		Spec: cmv1.IssuerSpec{
			IssuerConfig: cmv1.IssuerConfig{
				CA: &cmv1.CAIssuer{
					SecretName: "tls-ca-certificate",
				},
			},
		},
	}

	if opts != nil {
		for _, opt := range opts.IssuerOptions {
			opt(caIssuer)
		}
	}

	err = svc.SetDesiredKubeObject(caIssuer, serviceName+"-ca-issuer", kubeOpts...)
	if err != nil {
		err = fmt.Errorf("cannot create caIssuer object: %w", err)
		return serverCertsSecret, err
	}

	serverCert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "-server",
			Namespace: ns,
		},
		Spec: cmv1.CertificateSpec{
			SecretName: serverCertsSecret,
			Duration: &metav1.Duration{
				Duration: time.Duration(87600 * time.Hour),
			},
			RenewBefore: &metav1.Duration{
				Duration: time.Duration(2400 * time.Hour),
			},
			Subject: &cmv1.X509Subject{
				Organizations: []string{
					"vshn-appcat-server",
				},
			},
			IsCA: false,
			PrivateKey: &cmv1.CertificatePrivateKey{
				Algorithm: cmv1.ECDSAKeyAlgorithm,
				Encoding:  cmv1.PKCS1,
				Size:      256,
			},
			Usages: []cmv1.KeyUsage{"server auth", "client auth"},
			DNSNames: []string{
				serviceName + "." + ns + ".svc.cluster.local",
				serviceName + "." + ns + ".svc",
			},
			IssuerRef: certmgrv1.ObjectReference{
				Name:  serviceName + "-ca",
				Kind:  "Issuer",
				Group: "cert-manager.io",
			},
		},
	}

	if opts != nil {
		serverCert.Spec.DNSNames = append(serverCert.Spec.DNSNames, opts.AdditionalSans...)
	}

	cd := []xkube.ConnectionDetail{
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  ns,
				Name:       serverCertsSecret,
				FieldPath:  "data[ca.crt]",
			},
			ToConnectionSecretKey: "ca.crt",
		},
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  ns,
				Name:       serverCertsSecret,
				FieldPath:  "data[tls.crt]",
			},
			ToConnectionSecretKey: "tls.crt",
		},
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  ns,
				Name:       serverCertsSecret,
				FieldPath:  "data[tls.key]",
			},
			ToConnectionSecretKey: "tls.key",
		},
	}

	if opts != nil {
		for _, opt := range opts.CertOptions {
			opt(serverCert)
		}
	}

	serverCertOpts := append(kubeOpts, runtime.KubeOptionAddConnectionDetails(svc.GetCrossplaneNamespace(), cd...))

	err = svc.SetDesiredKubeObject(serverCert, serviceName+"-server-cert", serverCertOpts...)
	if err != nil {
		err = fmt.Errorf("cannot create serverCert object: %w", err)
		return serverCertsSecret, err
	}

	return serverCertsSecret, nil

}

// CreateMTLSCerts creates ssl/tls certificates with mutual authentication. Servicename will be concatenated with the given namespace to generate a proper k8s fqdn.
// In addition to error it returns the name of the server and client certificate secrets.
func CreateMTLSCerts(ctx context.Context, ns string, serviceName string, svc *runtime.ServiceRuntime, opts *TLSOptions) (string, string, error) {
	kubeOpts := []runtime.KubeObjectOption{}
	if opts != nil {
		kubeOpts = opts.KubeOptions
	}

	// Create self-signed issuer
	selfSignedIssuer := &cmv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "-selfsigned-issuer",
			Namespace: ns,
		},
		Spec: cmv1.IssuerSpec{
			IssuerConfig: cmv1.IssuerConfig{
				SelfSigned: &cmv1.SelfSignedIssuer{
					CRLDistributionPoints: []string{},
				},
			},
		},
	}

	if opts != nil {
		for _, opt := range opts.IssuerOptions {
			opt(selfSignedIssuer)
		}
	}

	err := svc.SetDesiredKubeObject(selfSignedIssuer, serviceName+"-selfsigned-issuer", kubeOpts...)
	if err != nil {
		err = fmt.Errorf("cannot create selfSignedIssuer object: %w", err)
		return "", "", err
	}

	serverCertsSecret := "tls-server-certificate"
	clientCertsSecret := "tls-client-certificate"

	// Create CA certificate
	caCert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "-ca",
			Namespace: ns,
		},
		Spec: cmv1.CertificateSpec{
			SecretName: "tls-ca-certificate",
			Duration: &metav1.Duration{
				Duration: time.Duration(87600 * time.Hour),
			},
			RenewBefore: &metav1.Duration{
				Duration: time.Duration(2400 * time.Hour),
			},
			Subject: &cmv1.X509Subject{
				Organizations: []string{
					"vshn-appcat-ca",
				},
			},
			IsCA: true,
			PrivateKey: &cmv1.CertificatePrivateKey{
				Algorithm: cmv1.RSAKeyAlgorithm,
				Encoding:  cmv1.PKCS1,
				Size:      4096,
			},
			CommonName: serviceName + "-ca",
			IssuerRef: certmgrv1.ObjectReference{
				Name:  serviceName + "-selfsigned-issuer",
				Kind:  "Issuer",
				Group: "cert-manager.io",
			},
		},
	}

	if opts != nil {
		for _, opt := range opts.CertOptions {
			opt(caCert)
		}
	}

	err = svc.SetDesiredKubeObject(caCert, serviceName+"-ca-cert", kubeOpts...)
	if err != nil {
		err = fmt.Errorf("cannot create caCert object: %w", err)
		return serverCertsSecret, clientCertsSecret, err
	}

	// Create CA issuer
	caIssuer := &cmv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "-ca-issuer",
			Namespace: ns,
		},
		Spec: cmv1.IssuerSpec{
			IssuerConfig: cmv1.IssuerConfig{
				CA: &cmv1.CAIssuer{
					SecretName: "tls-ca-certificate",
				},
			},
		},
	}

	if opts != nil {
		for _, opt := range opts.IssuerOptions {
			opt(caIssuer)
		}
	}

	err = svc.SetDesiredKubeObject(caIssuer, serviceName+"-ca-issuer", kubeOpts...)
	if err != nil {
		err = fmt.Errorf("cannot create caIssuer object: %w", err)
		return serverCertsSecret, clientCertsSecret, err
	}

	// Create server certificate
	serverCert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "-server",
			Namespace: ns,
		},
		Spec: cmv1.CertificateSpec{
			SecretName: serverCertsSecret,
			Duration: &metav1.Duration{
				Duration: time.Duration(87600 * time.Hour),
			},
			RenewBefore: &metav1.Duration{
				Duration: time.Duration(2400 * time.Hour),
			},
			Subject: &cmv1.X509Subject{
				Organizations: []string{
					"vshn-appcat-server",
				},
			},
			IsCA: false,
			PrivateKey: &cmv1.CertificatePrivateKey{
				Algorithm: cmv1.RSAKeyAlgorithm,
				Encoding:  cmv1.PKCS1,
				Size:      4096,
			},
			Usages: []cmv1.KeyUsage{"server auth", "client auth"},
			//DNSNames: []string{
			//	serviceName + "." + ns + ".svc.cluster.local",
			//	serviceName + "." + ns + ".svc",
			//},
			IssuerRef: certmgrv1.ObjectReference{
				Name:  serviceName + "-ca-issuer",
				Kind:  "Issuer",
				Group: "cert-manager.io",
			},
		},
	}

	if opts != nil {
		serverCert.Spec.DNSNames = append(serverCert.Spec.DNSNames, opts.AdditionalSans...)
		for _, opt := range opts.CertOptions {
			opt(serverCert)
		}
	}

	serverCertOpts := append(kubeOpts, runtime.KubeOptionAddConnectionDetails(svc.GetCrossplaneNamespace(),
		xkube.ConnectionDetail{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  ns,
				Name:       serverCertsSecret,
				FieldPath:  "data[ca.crt]",
			},
			ToConnectionSecretKey: "ca.crt",
		},
		xkube.ConnectionDetail{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  ns,
				Name:       serverCertsSecret,
				FieldPath:  "data[tls.crt]",
			},
			ToConnectionSecretKey: "tls.crt",
		},
		xkube.ConnectionDetail{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  ns,
				Name:       serverCertsSecret,
				FieldPath:  "data[tls.key]",
			},
			ToConnectionSecretKey: "tls.key",
		}))

	err = svc.SetDesiredKubeObject(serverCert, serviceName+"-server-cert", serverCertOpts...)
	if err != nil {
		err = fmt.Errorf("cannot create serverCert object: %w", err)
		return serverCertsSecret, clientCertsSecret, err
	}

	// Create client certificate
	clientCert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "-client",
			Namespace: ns,
		},
		Spec: cmv1.CertificateSpec{
			SecretName: clientCertsSecret,
			Duration: &metav1.Duration{
				Duration: time.Duration(87600 * time.Hour),
			},
			RenewBefore: &metav1.Duration{
				Duration: time.Duration(2400 * time.Hour),
			},
			Subject: &cmv1.X509Subject{
				Organizations: []string{
					"vshn-appcat-client",
				},
			},
			IsCA: false,
			//DNSNames: []string{
			//	serviceName + "." + ns + ".svc.cluster.local",
			//	serviceName + "." + ns + ".svc",
			//},
			PrivateKey: &cmv1.CertificatePrivateKey{
				Algorithm: cmv1.RSAKeyAlgorithm,
				Encoding:  cmv1.PKCS1,
				Size:      4096,
			},
			Usages: []cmv1.KeyUsage{"client auth"},
			IssuerRef: certmgrv1.ObjectReference{
				Name:  serviceName + "-ca-issuer",
				Kind:  "Issuer",
				Group: "cert-manager.io",
			},
		},
	}

	if opts != nil {
		clientCert.Spec.DNSNames = append(clientCert.Spec.DNSNames, opts.AdditionalSans...)
		for _, opt := range opts.CertOptions {
			opt(clientCert)
		}
	}

	clientCertOpts := append(kubeOpts, runtime.KubeOptionAddConnectionDetails(svc.GetCrossplaneNamespace(),
		xkube.ConnectionDetail{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  ns,
				Name:       clientCertsSecret,
				FieldPath:  "data[tls.crt]",
			},
			ToConnectionSecretKey: "client.crt",
		},
		xkube.ConnectionDetail{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  ns,
				Name:       clientCertsSecret,
				FieldPath:  "data[tls.key]",
			},
			ToConnectionSecretKey: "client.key",
		}))

	err = svc.SetDesiredKubeObject(clientCert, serviceName+"-client-cert", clientCertOpts...)
	if err != nil {
		err = fmt.Errorf("cannot create clientCert object: %w", err)
		return serverCertsSecret, clientCertsSecret, err
	}

	return serverCertsSecret, clientCertsSecret, nil
}
