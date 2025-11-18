package probes

import (
	"context"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/ory/dockertest"
)

func TestProbe(t *testing.T) {
	accessKey := "minioadmin"
	secretKey := "minioadmin"
	bucketName := "vshn-test-bucket-for-sli"
	useSSL := false // Set to true if your Minio server uses SSL

	t.Parallel()
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatal("can't create DockerPool")
	}

	minioServer, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "quay.io/minio/minio",
		Cmd: []string{
			"server",
			"/data",
			"--console-address",
			":9001",
		},
	})

	if err != nil {
		t.Fatal("Can't create minioServer")
	}

	endpoint := "127.0.0.1:" + minioServer.GetPort("9000/tcp")

	defer pool.Purge(minioServer)
	// Initialize a Minio client
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		t.Fatal("can't create minio client", err)
	}

	// Create a context
	ctx := context.Background()

	// Create the bucket
	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
		Region:        "us-east-1", // Replace with your desired region
		ObjectLocking: false,
	})
	if err != nil {
		// Check if the bucket already exists
		exists, err := minioClient.BucketExists(ctx, bucketName)
		if err == nil && exists {
			t.Fatalf("Bucket '%s' already exists\n", bucketName)
		} else {
			t.Log("issue during bucket creation")
			t.Fatal(err)
		}
	} else {
		t.Logf("Bucket '%s' created successfully\n", bucketName)
	}

	minio, err := NewMinio("VSHNMinio", "VSHNMinio", "default", "default", "VSHNMinio", "besteffort", "minio", endpoint, false, minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})

	if err != nil {
		t.Fatal("can't create Minio instance")
	}

	err = minio.Probe(context.Background())
	if err != nil {
		t.Fatal("can't probe")
	}
}
