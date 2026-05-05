package vshnopenbao

import (
	"context"
	"testing"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
)

func TestOpenBaoTLSSetup(t *testing.T) {
	svc, comp := getOpenBaoTestComp(t)

	result := SetupTLSCertificates(context.Background(), comp, svc)
	assert.Nil(t, result)

	// Test self-signed issuer
	selfSignedIssuer := &cmv1.Issuer{}
	err := svc.GetDesiredKubeObject(selfSignedIssuer, "openbao-test-selfsigned-issuer")
	assert.NoError(t, err)
	assert.NotNil(t, selfSignedIssuer.Spec.SelfSigned)
	assert.Equal(t, "vshn-openbao-openbao-test", selfSignedIssuer.Namespace)

	// Test root CA certificate
	rootCA := &cmv1.Certificate{}
	err = svc.GetDesiredKubeObject(rootCA, "openbao-test-ca-cert")
	assert.NoError(t, err)
	assert.Equal(t, "tls-ca-certificate", rootCA.Spec.SecretName)
	assert.True(t, rootCA.Spec.IsCA)
	assert.Equal(t, "openbao-test-selfsigned", rootCA.Spec.IssuerRef.Name)
	assert.Equal(t, "Issuer", rootCA.Spec.IssuerRef.Kind)

	// Test root CA issuer
	rootCAIssuer := &cmv1.Issuer{}
	err = svc.GetDesiredKubeObject(rootCAIssuer, "openbao-test-ca-issuer")
	assert.NoError(t, err)
	assert.NotNil(t, rootCAIssuer.Spec.CA)
	assert.Equal(t, "tls-ca-certificate", rootCAIssuer.Spec.CA.SecretName)

	// Test OpenBao server certificate
	serverCert := &cmv1.Certificate{}
	err = svc.GetDesiredKubeObject(serverCert, "openbao-test-server-cert")
	assert.NoError(t, err)
	assert.Equal(t, "tls-server-certificate", serverCert.Spec.SecretName)
	assert.Equal(t, "openbao-test-ca", serverCert.Spec.IssuerRef.Name)
	assert.Contains(t, serverCert.Spec.DNSNames, "openbao-test.vshn-openbao-openbao-test.svc.cluster.local")
	assert.Contains(t, serverCert.Spec.DNSNames, "openbao-test.vshn-openbao-openbao-test.svc")
	assert.Contains(t, serverCert.Spec.Usages, cmv1.KeyUsage("server auth"))
	assert.Contains(t, serverCert.Spec.Usages, cmv1.KeyUsage("client auth"))
}

func TestOpenBaoTLSConnectionDetails(t *testing.T) {
	svc, comp := getOpenBaoTestComp(t)

	result := SetupTLSCertificates(context.Background(), comp, svc)
	assert.Nil(t, result)

	obj := &xkube.Object{}
	err := svc.GetDesiredComposedResourceByName(obj, "openbao-test-server-cert")
	assert.NoError(t, err)
	require.NotNil(t, obj.Spec.ConnectionDetails)
	assert.Len(t, obj.Spec.ConnectionDetails, 3)

	// Verify ca.crt connection detail
	caCrtDetail := findConnectionDetail(obj.Spec.ConnectionDetails, "ca.crt")
	assert.NotNil(t, caCrtDetail)
	assert.Equal(t, "tls-server-certificate", caCrtDetail.ObjectReference.Name)
	assert.Equal(t, "data[ca.crt]", caCrtDetail.ObjectReference.FieldPath)

	// Verify tls.crt connection detail
	tlsCrtDetail := findConnectionDetail(obj.Spec.ConnectionDetails, "tls.crt")
	assert.NotNil(t, tlsCrtDetail)
	assert.Equal(t, "tls-server-certificate", tlsCrtDetail.ObjectReference.Name)
	assert.Equal(t, "data[tls.crt]", tlsCrtDetail.ObjectReference.FieldPath)

	// Verify tls.key connection detail
	tlsKeyDetail := findConnectionDetail(obj.Spec.ConnectionDetails, "tls.key")
	assert.NotNil(t, tlsKeyDetail)
	assert.Equal(t, "tls-server-certificate", tlsKeyDetail.ObjectReference.Name)
	assert.Equal(t, "data[tls.key]", tlsKeyDetail.ObjectReference.FieldPath)
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
