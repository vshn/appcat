package probes

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ Prober = &HTTP{}

type HTTP struct {
	url    string
	client *http.Client
	ProbeInfo
}

func NewHTTP(url string, tlsEnabled bool, cacert []byte, service, name, namespace, organization, servicelevel string, ha bool) *HTTP {
	transport := http.DefaultTransport
	if tlsEnabled {
		caPool := x509.NewCertPool()
		caPool.AppendCertsFromPEM(cacert)
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
			Service:       service,
			Name:          name,
			Namespace:     namespace,
			Organization:  organization,
			ServiceLevel:  servicelevel,
			HighAvailable: ha,
		},
	}
}

func (h *HTTP) Close() error {
	return nil
}

func (h *HTTP) GetInfo() ProbeInfo {
	return h.ProbeInfo
}

func (h *HTTP) Probe(ctx context.Context) error {

	l := log.FromContext(ctx).WithValues("http_prober", h.url)

	l.V(1).Info("Starting get request")
	resp, err := h.client.Get(h.url)
	if err != nil {
		return err
	}

	l.V(1).Info("Checking response")
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http request failed with status: %s", resp.Status)
	}

	return nil
}
