package probes

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ Prober = &HTTP{}

type HTTP struct {
	url    string
	client *http.Client
	ProbeInfo
}

func NewHTTP(url string, tlsEnabled bool, cacert *x509.Certificate, service, name, claimNamespace, instanceNamespace, organization, servicelevel, compositionName string, ha bool) *HTTP {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if tlsEnabled {
		caPool := x509.NewCertPool()
		caPool.AddCert(cacert)
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caPool,
			},
		}
	}
	return &HTTP{
		url: url,
		client: &http.Client{
			Transport: transport,
		},
		ProbeInfo: ProbeInfo{
			Service:           service,
			Name:              name,
			ClaimNamespace:    claimNamespace,
			InstanceNamespace: instanceNamespace,
			Organization:      organization,
			ServiceLevel:      servicelevel,
			HighAvailable:     ha,
			CompositionName:   compositionName,
		},
	}
}

func (h *HTTP) Close() error {
	// Release idle connections held by the transport.
	// Without this, stopping a probe leaves the transport's connection pool
	// open with connections to the remote endpoint.
	h.client.CloseIdleConnections()
	return nil
}

func (h *HTTP) GetInfo() ProbeInfo {
	return h.ProbeInfo
}

func (h *HTTP) Probe(ctx context.Context) error {

	l := log.FromContext(ctx).WithValues("http_prober", h.url)

	l.V(1).Info("Starting get request")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.url, nil)
	if err != nil {
		return err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}

	// Not closing response body prevents the HTTP transport from reusing the underlying
	// TCP connection. Draining before closing allows connection reuse.
	defer func() {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()
	}()

	l.V(1).Info("Checking response")
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http request failed with status: %s", resp.Status)
	}

	return nil
}
