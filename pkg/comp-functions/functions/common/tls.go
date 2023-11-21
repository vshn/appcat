package common

import (
	"context"
	"fmt"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	v1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateTlsCerts(ctx context.Context, ns string, instance string, svc *runtime.ServiceRuntime) error {

	selfSignedIssuer := &cmv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance + "-selfsigned",
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

	err := svc.SetDesiredKubeObject(selfSignedIssuer, instance+"-selfsigned-issuer")
	if err != nil {
		err = fmt.Errorf("cannot create selfSignedIssuer object: %w", err)
		return err
	}

	caCert := &cmv1.Certificate{

		ObjectMeta: metav1.ObjectMeta{
			Name:      instance + "-ca",
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
			CommonName: instance + "-ca",
			IssuerRef: v1.ObjectReference{
				Name:  instance + "-selfsigned",
				Kind:  "Issuer",
				Group: "cert-manager.io",
			},
		},
	}

	err = svc.SetDesiredKubeObject(caCert, instance+"-ca-cert")
	if err != nil {
		err = fmt.Errorf("cannot create caCert object: %w", err)
		return err
	}

	caIssuer := &cmv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance + "-ca",
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

	err = svc.SetDesiredKubeObject(caIssuer, instance+"-ca-issuer")
	if err != nil {
		err = fmt.Errorf("cannot create caIssuer object: %w", err)
		return err
	}

	serverCert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance + "-server",
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
				instance + "." + ns + ".svc.cluster.local",
				instance + "." + ns + ".svc",
			},
			IssuerRef: v1.ObjectReference{
				Name:  instance + "-ca",
				Kind:  "Issuer",
				Group: "cert-manager.io",
			},
		},
	}

	err = svc.SetDesiredKubeObject(serverCert, instance+"-server-cert")
	if err != nil {
		err = fmt.Errorf("cannot create serverCert object: %w", err)
		return err
	}

	return nil

}
