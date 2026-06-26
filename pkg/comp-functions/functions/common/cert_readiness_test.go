package common

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCertSANMatchers(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "redis"},
		DNSNames:     []string{"redis.example.com"},
		IPAddresses:  []net.IP{net.ParseIP("185.19.28.27")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	require.True(t, CertHasDNS("redis.example.com")(cert))
	require.False(t, CertHasDNS("other.example.com")(cert))
	require.True(t, CertHasIP("185.19.28.27")(cert))
	require.False(t, CertHasIP("1.2.3.4")(cert))
}
