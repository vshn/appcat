package probes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTP_Probe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	defer server.Close()

	httpProbe := NewHTTP(server.URL, false, nil, "service-1", "name-1", "namespace-1", "instance-1", "vshn", "besteffort", "cnpg", false)

	assert.NoError(t, httpProbe.Probe(context.TODO()))
}

func TestHTTP_Probe_TLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	defer server.Close()

	httpProbe := NewHTTP(server.URL, true, server.Certificate(), "mockservice", "name-1", "namespace-1", "instance-1", "vshn", "besteffort", "cnpg", false)

	assert.NoError(t, httpProbe.Probe(context.TODO()))
}
