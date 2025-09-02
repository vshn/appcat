package probes

import (
	"bytes"
	"context"

	miniolib "github.com/minio/minio-go/v7"
)

type VSHNMinio struct {
	minioClient       *miniolib.Client
	Service           string
	Name              string
	ClaimNamespace    string
	InstanceNamespace string
	HighAvailable     bool
	Organization      string
	ServiceLevel      string
}

func (minio VSHNMinio) Close() error {
	return nil
}

func (minio VSHNMinio) GetInfo() ProbeInfo {
	return ProbeInfo{
		Service:           minio.Service,
		Name:              minio.Name,
		ClaimNamespace:    minio.ClaimNamespace,
		InstanceNamespace: minio.InstanceNamespace,
		HighAvailable:     minio.HighAvailable,
		Organization:      minio.Organization,
		ServiceLevel:      minio.ServiceLevel,
	}
}

func (minio VSHNMinio) Probe(ctx context.Context) error {

	x := bytes.NewBufferString("This file is auto-generated, do not edit, VSHN SLI Exporter purposes")

	_, err := minio.minioClient.PutObject(context.Background(), "vshn-test-bucket-for-sli", "vshn-test-bucket-for-sli", x, int64(x.Len()), miniolib.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return err
	}

	return nil
}

func NewMinio(service, name, claimNamespace, instanceNamespace, organization, sla, endpointURL string, ha bool, opts miniolib.Options) (*VSHNMinio, error) {

	client, err := miniolib.New(endpointURL, &opts)
	if err != nil {
		return nil, err
	}

	return &VSHNMinio{
		minioClient:       client,
		Service:           service,
		Name:              name,
		ClaimNamespace:    claimNamespace,
		InstanceNamespace: instanceNamespace,
		HighAvailable:     ha,
		Organization:      organization,
		ServiceLevel:      sla,
	}, nil
}
