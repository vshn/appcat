package slareport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PDFUploader uploads pdfs to the configured S3 endpoint.
type PDFUploader struct {
	client *minio.Client
	bucket string
}

// PDF is contains all metainformation to upload a PDF to S3.
type PDF struct {
	Customer string
	Date     time.Time
	PDFData  io.ReadCloser
}

// NewPDFUploader initializes an S3 client.
func NewPDFUploader(ctx context.Context, endpoint, bucket, keyID, secretKey string) (PDFUploader, error) {

	log.FromContext(ctx).V(1).Info("Logging into S3 endpoint", "endpointurl", endpoint)

	p := PDFUploader{}

	url, err := url.Parse(endpoint)
	if err != nil {
		return p, nil
	}

	S3Client, err := minio.New(url.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(keyID, secretKey, ""),
		Secure: url.Scheme == "https",
	})
	if err != nil {
		return p, err
	}

	p.client = S3Client
	p.bucket = bucket
	return p, nil
}

// Upload uploads the given PDF to the logged in S3 enspoint.
// It will create an object with the pattern `year/month/customer.pdf`.
func (p *PDFUploader) Upload(ctx context.Context, pdf PDF) error {

	obj := fmt.Sprintf("%d/%s/%s.pdf", pdf.Date.Year(), pdf.Date.Month(), pdf.Customer)

	log.FromContext(ctx).V(1).Info("Uploading PDF", "object", obj)

	buf := &bytes.Buffer{}
	size, err := io.Copy(buf, pdf.PDFData)
	if err != nil {
		return err
	}

	_, err = p.client.PutObject(ctx, p.bucket, obj, buf, size, minio.PutObjectOptions{})
	return err
}
