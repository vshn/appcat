package probes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	miniolib "github.com/minio/minio-go/v7"
)

type VSHNGarage struct {
	minioClient       *miniolib.Client
	httpClient        *http.Client
	adminURL          string
	adminToken        string
	bucketName        string
	Service           string
	Name              string
	ClaimNamespace    string
	InstanceNamespace string
	HighAvailable     bool
	Organization      string
	ServiceLevel      string
	CompositionName   string
}

func (g VSHNGarage) Close() error {
	return nil
}

func (g VSHNGarage) GetInfo() ProbeInfo {
	return ProbeInfo{
		Service:           g.Service,
		Name:              g.Name,
		ClaimNamespace:    g.ClaimNamespace,
		InstanceNamespace: g.InstanceNamespace,
		HighAvailable:     g.HighAvailable,
		Organization:      g.Organization,
		ServiceLevel:      g.ServiceLevel,
		CompositionName:   g.CompositionName,
	}
}

func (g VSHNGarage) Probe(ctx context.Context) error {
	if err := g.checkAdminHealth(ctx); err != nil {
		return err
	}
	return g.checkS3Write(ctx)
}

func (g VSHNGarage) checkAdminHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.adminURL+"/v2/GetClusterHealth", nil)
	if err != nil {
		return fmt.Errorf("cannot create admin health request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.adminToken)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("admin health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("admin API returned status %d: %s", resp.StatusCode, body)
	}

	var health struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return fmt.Errorf("cannot decode admin health response: %w", err)
	}
	if health.Status != "healthy" {
		return fmt.Errorf("garage cluster health is %q", health.Status)
	}
	return nil
}

func (g VSHNGarage) checkS3Write(ctx context.Context) error {
	x := bytes.NewBufferString("This file is auto-generated, do not edit, VSHN SLI Exporter purposes")
	_, err := g.minioClient.PutObject(ctx, g.bucketName, "vshn-sli-probe", x, int64(x.Len()), miniolib.PutObjectOptions{ContentType: "application/octet-stream"})
	return err
}

func NewGarage(service, name, claimNamespace, instanceNamespace, organization, sla, compositionName, endpointURL, adminToken, bucketName string, ha bool, opts miniolib.Options) (*VSHNGarage, error) {
	u, err := url.Parse(endpointURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse garage endpoint %q: %w", endpointURL, err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("garage endpoint %q has no host (add scheme, e.g. http://)", endpointURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("garage endpoint %q has unsupported scheme %q (must be http or https)", endpointURL, u.Scheme)
	}

	opts.Secure = u.Scheme == "https"
	minioClient, err := miniolib.New(u.Host, &opts)
	if err != nil {
		return nil, err
	}

	adminURL := fmt.Sprintf("%s://%s:3903", u.Scheme, u.Hostname())

	return &VSHNGarage{
		minioClient:       minioClient,
		httpClient:        &http.Client{Timeout: 5 * time.Second},
		adminURL:          adminURL,
		adminToken:        adminToken,
		bucketName:        bucketName,
		Service:           service,
		Name:              name,
		ClaimNamespace:    claimNamespace,
		InstanceNamespace: instanceNamespace,
		HighAvailable:     ha,
		Organization:      organization,
		ServiceLevel:      sla,
		CompositionName:   compositionName,
	}, nil
}
