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

func CreateTlsCerts(ctx context.Context, ns string, serviceName string, svc *runtime.ServiceRuntime) error {

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

	err := svc.SetDesiredKubeObject(selfSignedIssuer, serviceName+"-selfsigned-issuer")
	if err != nil {
		err = fmt.Errorf("cannot create selfSignedIssuer object: %w", err)
		return err
	}

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

	err = svc.SetDesiredKubeObject(caCert, serviceName+"-ca-cert")
	if err != nil {
		err = fmt.Errorf("cannot create caCert object: %w", err)
		return err
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

	err = svc.SetDesiredKubeObject(caIssuer, serviceName+"-ca-issuer")
	if err != nil {
		err = fmt.Errorf("cannot create caIssuer object: %w", err)
		return err
	}

	serverCert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "-server",
			Namespace: ns,
		},
		Spec: cmv1.CertificateSpec{
			SecretName: "tls-server-certificate",
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

	cd := []xkube.ConnectionDetail{
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  ns,
				Name:       "tls-server-certificate",
				FieldPath:  "data[ca.crt]",
			},
			ToConnectionSecretKey: "ca.crt",
		},
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  ns,
				Name:       "tls-server-certificate",
				FieldPath:  "data[tls.crt]",
			},
			ToConnectionSecretKey: "tls.crt",
		},
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  ns,
				Name:       "tls-server-certificate",
				FieldPath:  "data[tls.key]",
			},
			ToConnectionSecretKey: "tls.key",
		},
	}

	err = svc.SetDesiredKubeObject(serverCert, serviceName+"-server-cert", runtime.KubeOptionAddConnectionDetails(ns, cd...))
	if err != nil {
		err = fmt.Errorf("cannot create serverCert object: %w", err)
		return err
	}

	return nil

}
