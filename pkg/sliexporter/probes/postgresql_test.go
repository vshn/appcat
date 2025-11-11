package probes

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgreSQL_Probe(t *testing.T) {
	t.Parallel()

	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	user := "foo"
	password := "bar"
	db := "buzz"

	res, err := pool.Run("ghcr.io/vshn/postgres", "16.10", []string{
		"POSTGRES_USER=" + user,
		"POSTGRES_PASSWORD=" + password,
		"POSTGRES_DB=" + db,
	})
	require.NoError(t, err)
	defer pool.Purge(res)

	var p Prober
	assert.Eventually(t, func() bool {
		p, err = NewPostgreSQL("VSHN", "test", "test", "test",
			fmt.Sprintf(
				"postgresql://%s:%s@%s:%s/%s?sslmode=disable",
				user,
				password,
				"localhost",
				res.GetPort("5432/tcp"),
				db,
			),
			"test", "vshnpostgrescnpg.vshn.appcat.vshn.io", "besteffort", false,
		)
		return err == nil
	}, 2*time.Second, 500*time.Millisecond)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		err = p.Probe(context.TODO())
		return err == nil
	}, 20*time.Second, 500*time.Millisecond)
	assert.NoError(t, err)

	assert.Never(t, func() bool {
		err = p.Probe(context.TODO())
		return err != nil
	}, 1*time.Second, 100*time.Millisecond)

	require.NoError(t, res.Close())

	assert.Eventually(t, func() bool {
		err = p.Probe(context.TODO())
		return err != nil
	}, 2*time.Second, 500*time.Millisecond)
	assert.Error(t, err)

	assert.Never(t, func() bool {
		err = p.Probe(context.TODO())
		return err == nil
	}, 1*time.Second, 100*time.Millisecond)

	assert.NoError(t, p.Close())

}

func TestPostgreSQL_Fail(t *testing.T) {
	t.Parallel()
	p, err := NewFailingPostgreSQL("FAKE", "foo", "bar", "bar")
	require.NoError(t, err)

	require.NotPanics(t, func() {
		assert.Error(t, p.Probe(context.Background()))

		pi := p.GetInfo()
		assert.Equal(t, pi.Service, "FAKE")
		assert.Equal(t, pi.Name, "foo")
		assert.Equal(t, pi.ClaimNamespace, "bar")
		assert.Equal(t, pi.InstanceNamespace, "bar")
		assert.NoError(t, p.Close())
	})
}

func TestPGWithCA(t *testing.T) {
	t.Parallel()
	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Println("Failed to generate private key:", err)
		t.FailNow()
		return
	}

	// Create a template for the certificate
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		DNSNames:     []string{"localhost"},
		Subject: pkix.Name{
			Organization: []string{"Example Corp"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Valid for one year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create the certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		fmt.Println("Failed to create certificate:", err)
		t.FailNow()
		return
	}

	pembytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})

	op := PGWithCA(pembytes, true)

	conf, err := pgxpool.ParseConfig("postgresql://foo:bar@localhost:5432/buzz?sslmode=disable")
	require.NoError(t, err)

	err = op(conf)
	assert.NoError(t, err)
	assert.NotNil(t, conf.ConnConfig.TLSConfig)
	assert.NotNil(t, conf.ConnConfig.TLSConfig.RootCAs)
}

func TestPGWithCA_DisabledTLS(t *testing.T) {
	t.Parallel()
	op := PGWithCA(nil, false)

	conf, err := pgxpool.ParseConfig("postgresql://foo:bar@localhost:5432/buzz?sslmode=disable")
	require.NoError(t, err)

	err = op(conf)
	assert.NoError(t, err)
	assert.Nil(t, conf.ConnConfig.TLSConfig)
}

func TestPGWithCA_InvalidCA(t *testing.T) {
	t.Parallel()
	op := PGWithCA(nil, true)

	conf, err := pgxpool.ParseConfig("postgresql://foo:bar@localhost:5432/buzz?sslmode=disable")
	require.NoError(t, err)

	err = op(conf)
	assert.Error(t, err)
	assert.Equal(t, "got nil CA", err.Error())
}
