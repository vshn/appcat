package miniobucket

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
)

func TestProvisionMiniobucket(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "miniobucket/bucket.yaml")

	ctx := context.TODO()

	bucketName := "mytest"

	res := ProvisionMiniobucket(ctx, &v1.ObjectBucket{}, svc)
	assert.Nil(t, res)

	bucket := &miniov1.Bucket{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(bucket, "minio-bucket"))
	assert.Equal(t, bucketName, bucket.GetName())

	user := &miniov1.User{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(user, "minio-user"))
	assert.Equal(t, bucketName, user.GetName())
	assert.Contains(t, user.Spec.ForProvider.Policies, bucketName)

	policy := &miniov1.Policy{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(policy, "minio-policy"))
	assert.Equal(t, bucketName, policy.GetName())

}

// TestBucketWithoutName tests that when bucketName is not specified, it uses the composite name
func TestBucketWithoutName(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "miniobucket/bucket-no-name.yaml")

	ctx := context.TODO()
	compositeName := "testbucket-minio123"

	// Get the composite bucket to verify bucketName gets populated
	compositeBucket := &v1.ObjectBucket{}
	err := svc.GetObservedComposite(compositeBucket)
	assert.NoError(t, err)

	res := ProvisionMiniobucket(ctx, compositeBucket, svc)
	assert.Nil(t, res)

	// Verify that bucketName was populated in the composite status
	assert.Equal(t, compositeName, compositeBucket.Status.BucketName)

	bucket := &miniov1.Bucket{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(bucket, "minio-bucket"))
	assert.Equal(t, compositeName, bucket.GetName())

	user := &miniov1.User{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(user, "minio-user"))
	assert.Equal(t, compositeName, user.GetName())
	assert.Contains(t, user.Spec.ForProvider.Policies, compositeName)

	policy := &miniov1.Policy{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(policy, "minio-policy"))
	assert.Equal(t, compositeName, policy.GetName())
	assert.Equal(t, compositeName, policy.Spec.ForProvider.AllowBucket)
}
