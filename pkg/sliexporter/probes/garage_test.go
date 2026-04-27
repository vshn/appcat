package probes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	miniolib "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// garageWithAdmin builds a VSHNGarage whose admin URL points to the given test
// server and whose S3 client points to s3Host. Keeps test setup minimal.
func garageWithAdmin(adminServer *httptest.Server, s3Host string) VSHNGarage {
	mc, _ := miniolib.New(s3Host, &miniolib.Options{
		Creds: credentials.NewStaticV4("key", "secret", ""),
	})
	return VSHNGarage{
		httpClient:  &http.Client{},
		adminURL:    adminServer.URL,
		adminToken:  "test-token",
		bucketName:  "test-bucket",
		minioClient: mc,
	}
}

// TestNewGarage_EndpointParsing covers URL validation and admin URL derivation.

func TestNewGarage_MissingScheme(t *testing.T) {
	opts := miniolib.Options{Creds: credentials.NewStaticV4("k", "s", "")}
	_, err := NewGarage("svc", "n", "ns", "ins", "org", "be", "comp", "s3.example.com:3900", "tok", "bkt", false, opts)
	assert.ErrorContains(t, err, "has no host")
}

func TestNewGarage_EmptyEndpoint(t *testing.T) {
	opts := miniolib.Options{Creds: credentials.NewStaticV4("k", "s", "")}
	_, err := NewGarage("svc", "n", "ns", "ins", "org", "be", "comp", "", "tok", "bkt", false, opts)
	assert.ErrorContains(t, err, "has no host")
}

func TestNewGarage_MissingSchemeWithHost(t *testing.T) {
	opts := miniolib.Options{Creds: credentials.NewStaticV4("k", "s", "")}
	_, err := NewGarage("svc", "n", "ns", "ins", "org", "be", "comp", "//s3.example.com:3900", "tok", "bkt", false, opts)
	assert.ErrorContains(t, err, "unsupported scheme")
}

func TestNewGarage_UnsupportedScheme(t *testing.T) {
	opts := miniolib.Options{Creds: credentials.NewStaticV4("k", "s", "")}
	_, err := NewGarage("svc", "n", "ns", "ins", "org", "be", "comp", "s3://bucket.example.com", "tok", "bkt", false, opts)
	assert.ErrorContains(t, err, "unsupported scheme")
}

func TestNewGarage_AdminURLDerivedFromEndpoint(t *testing.T) {
	opts := miniolib.Options{Creds: credentials.NewStaticV4("k", "s", "")}
	g, err := NewGarage("svc", "n", "ns", "ins", "org", "be", "comp", "http://s3.example.com:3900", "tok", "bkt", false, opts)
	require.NoError(t, err)
	assert.Equal(t, "http://s3.example.com:3903", g.adminURL)
}

func TestNewGarage_HTTPSEndpoint(t *testing.T) {
	opts := miniolib.Options{Creds: credentials.NewStaticV4("k", "s", "")}
	g, err := NewGarage("svc", "n", "ns", "ins", "org", "be", "comp", "https://s3.example.com:3900", "tok", "bkt", false, opts)
	require.NoError(t, err)
	assert.Equal(t, "https://s3.example.com:3903", g.adminURL)
}

// TestGarage_Probe_* covers Probe error paths.

func TestGarage_Probe_AdminNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	g := garageWithAdmin(server, "localhost:1")
	err := g.Probe(context.Background())
	assert.ErrorContains(t, err, "503")
}

func TestGarage_Probe_AdminUnhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "degraded"})
	}))
	defer server.Close()

	g := garageWithAdmin(server, "localhost:1")
	err := g.Probe(context.Background())
	assert.ErrorContains(t, err, `"degraded"`)
}

func TestGarage_Probe_AdminInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	g := garageWithAdmin(server, "localhost:1")
	err := g.Probe(context.Background())
	assert.ErrorContains(t, err, "cannot decode admin health response")
}

func TestGarage_Probe_S3WriteFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	}))
	defer server.Close()

	// Admin health passes; S3 client targets a port with nothing listening.
	g := garageWithAdmin(server, "localhost:1")
	err := g.Probe(context.Background())
	assert.Error(t, err)
}

func TestGarage_Probe_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mc, _ := miniolib.New("localhost:1", &miniolib.Options{
		Creds: credentials.NewStaticV4("k", "s", ""),
	})
	g := VSHNGarage{
		httpClient:  &http.Client{},
		adminURL:    "http://localhost:1",
		adminToken:  "token",
		bucketName:  "test-bucket",
		minioClient: mc,
	}

	err := g.Probe(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}
