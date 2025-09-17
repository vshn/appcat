package exoscalebucket

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	cloudscalev1 "github.com/vshn/provider-cloudscale/apis/cloudscale/v1"
	exoscalev1 "github.com/vshn/provider-exoscale/apis/exoscale/v1"
)

func TestProvisionCloudscalebucket(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "exoscalebucket/bucket.yaml")

	ctx := context.TODO()

	bucketName := "mytest"

	res := ProvisionExoscalebucket(ctx, &v1.ObjectBucket{}, svc)
	assert.Nil(t, res)

	bucket := &exoscalev1.Bucket{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(bucket, "exoscale-bucket"))
	assert.Equal(t, bucketName, bucket.GetName())

	user := &exoscalev1.IAMKey{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(user, "exoscale-iam"))
	assert.Equal(t, bucketName, user.GetName())

}

// GivenObservedBucket_ThenExpectNameFromObserved
func TestExistingBuckets(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "exoscalebucket/bucket-existing.yaml")

	ctx := context.TODO()

	res := ProvisionExoscalebucket(ctx, &v1.ObjectBucket{}, svc)
	assert.Nil(t, res)

	bucket := &cloudscalev1.Bucket{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(bucket, "exoscale-bucket"))
	assert.Equal(t, "existing-bucket", bucket.GetName())

	user := &cloudscalev1.ObjectsUser{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(user, "exoscale-iam"))
	assert.Equal(t, "existing-iam", user.GetName())

}

// TestBucketWithoutName tests that when bucketName is not specified, it uses the composite name
func TestBucketWithoutName(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "exoscalebucket/bucket-no-name.yaml")

	ctx := context.TODO()
	compositeName := "testbucket-xyz123"

	// Get the composite bucket to verify bucketName gets populated
	compositeBucket := &v1.ObjectBucket{}
	err := svc.GetObservedComposite(compositeBucket)
	assert.NoError(t, err)

	res := ProvisionExoscalebucket(ctx, compositeBucket, svc)
	assert.Nil(t, res)

	// Verify that bucketName was populated in the composite status
	assert.Equal(t, compositeName, compositeBucket.Status.BucketName)

	bucket := &exoscalev1.Bucket{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(bucket, "exoscale-bucket"))
	assert.Equal(t, compositeName, bucket.GetName())
	assert.Equal(t, compositeName, bucket.Spec.ForProvider.BucketName)

	user := &exoscalev1.IAMKey{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(user, "exoscale-iam"))
	assert.Equal(t, compositeName, user.GetName())
	assert.Contains(t, user.Spec.ForProvider.Services.SOS.Buckets, compositeName)
}
