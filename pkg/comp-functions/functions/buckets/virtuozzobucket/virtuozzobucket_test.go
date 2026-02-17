package virtuozzobucket

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	virtuozzov1 "github.com/vshn/provider-virtuozzo/apis/virtuozzo/v1"
)

func TestProvisionVirtuozzobucket(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "virtuozzobucket/bucket.yaml")

	ctx := context.TODO()

	bucketName := "mytest"

	res := ProvisionVirtuozzobucket(ctx, &v1.ObjectBucket{}, svc)
	assert.Nil(t, res)

	bucket := &virtuozzov1.Bucket{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(bucket, "virtuozzo-bucket"))
	assert.Equal(t, bucketName, bucket.GetName())
}

// GivenObservedBucket_ThenExpectNameFromObserved
func TestExistingBuckets(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "virtuozzobucket/bucket-existing.yaml")

	ctx := context.TODO()

	res := ProvisionVirtuozzobucket(ctx, &v1.ObjectBucket{}, svc)
	assert.Nil(t, res)

	bucket := &virtuozzov1.Bucket{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(bucket, "virtuozzo-bucket"))
	assert.Equal(t, "existing-bucket", bucket.GetName())
}

// TestBucketWithoutName tests that when bucketName is not specified, it uses the composite name
func TestBucketWithoutName(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "virtuozzobucket/bucket-no-name.yaml")

	ctx := context.TODO()
	compositeName := "testbucket-abc789"

	compositeBucket := &v1.ObjectBucket{}
	err := svc.GetObservedComposite(compositeBucket)
	assert.NoError(t, err)

	res := ProvisionVirtuozzobucket(ctx, compositeBucket, svc)
	assert.Nil(t, res)

	assert.Equal(t, compositeName, compositeBucket.Status.BucketName)

	bucket := &virtuozzov1.Bucket{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(bucket, "virtuozzo-bucket"))
	assert.Equal(t, compositeName, bucket.GetName())
	assert.Equal(t, compositeName, bucket.Spec.ForProvider.BucketName)
}
