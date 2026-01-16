package vshnopenbao

import (
	"testing"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
)

func TestEnableOpenBaoTLSSupport(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnopenbao/deploy/01_default.yaml")

	comp := &vshnv1.VSHNOpenBao{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	err = enableOpenBaoTLSSupport(comp, svc)
	assert.NoError(t, err)

	serviceName := comp.GetName()
	ns := comp.GetInstanceNamespace()

	// Test self-signed issuer
	selfSignedIssuer := &cmv1.Issuer{}
	err = svc.GetDesiredKubeObject(selfSignedIssuer, serviceName+"-selfsigned-issuer")
	assert.NoError(t, err)
	assert.NotNil(t, selfSignedIssuer.Spec.SelfSigned)
	assert.Equal(t, ns, selfSignedIssuer.Namespace)

	// Test root CA certificate
	rootCA := &cmv1.Certificate{}
	err = svc.GetDesiredKubeObject(rootCA, serviceName+"-ca")
	assert.NoError(t, err)
	assert.Equal(t, serviceName+"-ca-tls", rootCA.Spec.SecretName)
	assert.True(t, rootCA.Spec.IsCA)
	assert.Equal(t, serviceName+"-selfsigned-issuer", rootCA.Spec.IssuerRef.Name)
	assert.Equal(t, "Issuer", rootCA.Spec.IssuerRef.Kind)
	assert.Equal(t, time.Duration(87600*time.Hour), rootCA.Spec.Duration.Duration)
	assert.Equal(t, time.Duration(2400*time.Hour), rootCA.Spec.RenewBefore.Duration)

	// Test root CA issuer
	rootCAIssuer := &cmv1.Issuer{}
	err = svc.GetDesiredKubeObject(rootCAIssuer, serviceName+"-ca-issuer")
	assert.NoError(t, err)
	assert.NotNil(t, rootCAIssuer.Spec.CA)
	assert.Equal(t, serviceName+"-ca-tls", rootCAIssuer.Spec.CA.SecretName)

	// Test OpenBao server certificate
	serverCert := &cmv1.Certificate{}
	err = svc.GetDesiredKubeObject(serverCert, serviceName+"-server")
	assert.NoError(t, err)
	assert.Equal(t, serviceName+"-server-tls", serverCert.Spec.SecretName)
	assert.Equal(t, serviceName+"-ca-issuer", serverCert.Spec.IssuerRef.Name)
	assert.Contains(t, serverCert.Spec.DNSNames, serviceName+"."+ns+".svc.cluster.local")
	assert.Contains(t, serverCert.Spec.DNSNames, serviceName+"."+ns+".svc")
	assert.Contains(t, serverCert.Spec.Usages, cmv1.KeyUsage("server auth"))
	assert.Contains(t, serverCert.Spec.Usages, cmv1.KeyUsage("client auth"))

	// Test connection details
	obj := &xkube.Object{}
	err = svc.GetDesiredComposedResourceByName(obj, serviceName+"-server")
	assert.NoError(t, err)
	require.NotNil(t, obj.Spec.ConnectionDetails)
	assert.Len(t, obj.Spec.ConnectionDetails, 3)

	// Verify ca.crt connection detail
	caCrtDetail := findConnectionDetail(obj.Spec.ConnectionDetails, "ca.crt")
	assert.NotNil(t, caCrtDetail)
	assert.Equal(t, serviceName+"-server-tls", caCrtDetail.ObjectReference.Name)
	assert.Equal(t, "data[ca.crt]", caCrtDetail.ObjectReference.FieldPath)

	// Verify tls.crt connection detail
	tlsCrtDetail := findConnectionDetail(obj.Spec.ConnectionDetails, "tls.crt")
	assert.NotNil(t, tlsCrtDetail)
	assert.Equal(t, serviceName+"-server-tls", tlsCrtDetail.ObjectReference.Name)
	assert.Equal(t, "data[tls.crt]", tlsCrtDetail.ObjectReference.FieldPath)

	// Verify tls.key connection detail
	tlsKeyDetail := findConnectionDetail(obj.Spec.ConnectionDetails, "tls.key")
	assert.NotNil(t, tlsKeyDetail)
	assert.Equal(t, serviceName+"-server-tls", tlsKeyDetail.ObjectReference.Name)
	assert.Equal(t, "data[tls.key]", tlsKeyDetail.ObjectReference.FieldPath)
}

func TestCreateIssuer(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnopenbao/deploy/01_default.yaml")

	comp := &vshnv1.VSHNOpenBao{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	ns := comp.GetInstanceNamespace()

	tests := []struct {
		name         string
		issuerName   string
		setupOpts    func() *common.TLSOptions
		validateFunc func(*testing.T, *cmv1.Issuer)
	}{
		{
			name:       "self-signed issuer",
			issuerName: "test-selfsigned",
			setupOpts: func() *common.TLSOptions {
				return &common.TLSOptions{
					IssuerOptions: []common.IssuerOption{
						withIssuerOfTypeSelfSigned(),
					},
				}
			},
			validateFunc: func(t *testing.T, issuer *cmv1.Issuer) {
				assert.NotNil(t, issuer.Spec.SelfSigned)
				assert.Equal(t, []string{}, issuer.Spec.SelfSigned.CRLDistributionPoints)
			},
		},
		{
			name:       "CA issuer",
			issuerName: "test-ca",
			setupOpts: func() *common.TLSOptions {
				return &common.TLSOptions{
					IssuerOptions: []common.IssuerOption{
						withIssuerOfTypeCA("test-secret"),
					},
				}
			},
			validateFunc: func(t *testing.T, issuer *cmv1.Issuer) {
				assert.NotNil(t, issuer.Spec.CA)
				assert.Equal(t, "test-secret", issuer.Spec.CA.SecretName)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.setupOpts()
			issuer := createIssuer(ns, tt.issuerName, opts)

			assert.Equal(t, tt.issuerName, issuer.Name)
			assert.Equal(t, ns, issuer.Namespace)
			tt.validateFunc(t, issuer)
		})
	}
}

func TestCreateCertificate(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnopenbao/deploy/01_default.yaml")

	comp := &vshnv1.VSHNOpenBao{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	ns := comp.GetInstanceNamespace()

	tests := []struct {
		name         string
		certName     string
		setupOpts    func() *common.TLSOptions
		validateFunc func(*testing.T, *cmv1.Certificate)
	}{
		{
			name:     "basic certificate",
			certName: "test-cert",
			setupOpts: func() *common.TLSOptions {
				return &common.TLSOptions{
					CertOptions: []common.CertOptions{
						withCertificateSecretName("test-secret"),
					},
				}
			},
			validateFunc: func(t *testing.T, cert *cmv1.Certificate) {
				assert.Equal(t, "test-secret", cert.Spec.SecretName)
				assert.Equal(t, cmv1.RSAKeyAlgorithm, cert.Spec.PrivateKey.Algorithm)
				assert.Equal(t, cmv1.PKCS1, cert.Spec.PrivateKey.Encoding)
				assert.Equal(t, 4096, cert.Spec.PrivateKey.Size)
			},
		},
		{
			name:     "CA certificate",
			certName: "test-ca",
			setupOpts: func() *common.TLSOptions {
				return &common.TLSOptions{
					CertOptions: []common.CertOptions{
						withCertificateSecretName("test-ca-secret"),
						withCertificateIsCA(true),
						withCertificateDuration(time.Duration(87600 * time.Hour)),
						withCertificateRenewBefore(time.Duration(2400 * time.Hour)),
						withCertificateIssuerRef("selfsigned-issuer"),
					},
				}
			},
			validateFunc: func(t *testing.T, cert *cmv1.Certificate) {
				assert.Equal(t, "test-ca-secret", cert.Spec.SecretName)
				assert.True(t, cert.Spec.IsCA)
				assert.Equal(t, time.Duration(87600*time.Hour), cert.Spec.Duration.Duration)
				assert.Equal(t, time.Duration(2400*time.Hour), cert.Spec.RenewBefore.Duration)
				assert.Equal(t, "selfsigned-issuer", cert.Spec.IssuerRef.Name)
			},
		},
		{
			name:     "server certificate with DNS names",
			certName: "test-server",
			setupOpts: func() *common.TLSOptions {
				return &common.TLSOptions{
					CertOptions: []common.CertOptions{
						withCertificateSecretName("test-server-secret"),
						withCertificateDNSName("service.namespace.svc.cluster.local"),
						withCertificateDNSName("service.namespace.svc"),
						withCertificateUsages([]cmv1.KeyUsage{"server auth", "client auth"}),
						withCertificateIssuerRef("ca-issuer"),
					},
				}
			},
			validateFunc: func(t *testing.T, cert *cmv1.Certificate) {
				assert.Equal(t, "test-server-secret", cert.Spec.SecretName)
				assert.Contains(t, cert.Spec.DNSNames, "service.namespace.svc.cluster.local")
				assert.Contains(t, cert.Spec.DNSNames, "service.namespace.svc")
				assert.Contains(t, cert.Spec.Usages, cmv1.KeyUsage("server auth"))
				assert.Contains(t, cert.Spec.Usages, cmv1.KeyUsage("client auth"))
				assert.Equal(t, "ca-issuer", cert.Spec.IssuerRef.Name)
				assert.Equal(t, "Issuer", cert.Spec.IssuerRef.Kind)
				assert.Equal(t, "cert-manager.io", cert.Spec.IssuerRef.Group)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.setupOpts()
			cert := createCertificate(ns, tt.certName, opts)

			assert.Equal(t, tt.certName, cert.Name)
			assert.Equal(t, ns, cert.Namespace)
			assert.Equal(t, tt.certName+"-ca", cert.Spec.CommonName)
			assert.Contains(t, cert.Spec.Subject.Organizations, "vshn-appcat-ca")
			tt.validateFunc(t, cert)
		})
	}
}

func TestCertificateOptions(t *testing.T) {
	cert := &cmv1.Certificate{
		Spec: cmv1.CertificateSpec{},
	}

	// Test withCertificateSecretName
	withCertificateSecretName("test-secret")(cert)
	assert.Equal(t, "test-secret", cert.Spec.SecretName)

	// Test withCertificateDNSName
	withCertificateDNSName("test1.example.com")(cert)
	withCertificateDNSName("test2.example.com")(cert)
	assert.Len(t, cert.Spec.DNSNames, 2)
	assert.Contains(t, cert.Spec.DNSNames, "test1.example.com")
	assert.Contains(t, cert.Spec.DNSNames, "test2.example.com")

	// Test withCertificateIsCA
	withCertificateIsCA(true)(cert)
	assert.True(t, cert.Spec.IsCA)

	// Test withCertificateIssuerRef
	withCertificateIssuerRef("test-issuer")(cert)
	assert.Equal(t, "test-issuer", cert.Spec.IssuerRef.Name)
	assert.Equal(t, "Issuer", cert.Spec.IssuerRef.Kind)
	assert.Equal(t, "cert-manager.io", cert.Spec.IssuerRef.Group)

	// Test withCertificateDuration
	duration := time.Duration(87600 * time.Hour)
	withCertificateDuration(duration)(cert)
	assert.Equal(t, duration, cert.Spec.Duration.Duration)

	// Test withCertificateRenewBefore
	renewBefore := time.Duration(2400 * time.Hour)
	withCertificateRenewBefore(renewBefore)(cert)
	assert.Equal(t, renewBefore, cert.Spec.RenewBefore.Duration)

	// Test withCertificateUsages
	usages := []cmv1.KeyUsage{"server auth", "client auth"}
	withCertificateUsages(usages)(cert)
	assert.Equal(t, usages, cert.Spec.Usages)
}

func TestIssuerOptions(t *testing.T) {
	// Test withIssuerOfTypeSelfSigned
	issuer := &cmv1.Issuer{}
	withIssuerOfTypeSelfSigned()(issuer)
	assert.NotNil(t, issuer.Spec.SelfSigned)
	assert.Equal(t, []string{}, issuer.Spec.SelfSigned.CRLDistributionPoints)

	// Test withIssuerOfTypeCA
	issuer2 := &cmv1.Issuer{}
	withIssuerOfTypeCA("ca-secret")(issuer2)
	assert.NotNil(t, issuer2.Spec.CA)
	assert.Equal(t, "ca-secret", issuer2.Spec.CA.SecretName)
}

// Helper function to find connection detail by key
func findConnectionDetail(details []xkube.ConnectionDetail, key string) *xkube.ConnectionDetail {
	for _, detail := range details {
		if detail.ToConnectionSecretKey == key {
			return &detail
		}
	}
	return nil
}
