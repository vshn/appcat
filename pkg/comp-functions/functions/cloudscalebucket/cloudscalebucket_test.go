package cloudscalebucket

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	cloudscalev1 "github.com/vshn/provider-cloudscale/apis/cloudscale/v1"
)

func TestProvisionCloudscalebucket(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "cloudscalebucket/bucket.yaml")

	ctx := context.TODO()

	bucketName := "mytest"

	res := ProvisionCloudscalebucket(ctx, svc)
	assert.Nil(t, res)

	bucket := &cloudscalev1.Bucket{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(bucket, "cloudscale-bucket"))
	assert.Equal(t, bucketName, bucket.GetName())

	user := &cloudscalev1.ObjectsUser{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(user, "cloudscale-user"))
	assert.Equal(t, bucketName, user.GetName())

}

// GivenObservedBucket_ThenExpectNameFromObserved
func TestExistingBuckets(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "cloudscalebucket/bucket-existing.yaml")

	ctx := context.TODO()

	res := ProvisionCloudscalebucket(ctx, svc)
	assert.Nil(t, res)

	bucket := &cloudscalev1.Bucket{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(bucket, "cloudscale-bucket"))
	assert.Equal(t, "existing-bucket", bucket.GetName())

	user := &cloudscalev1.ObjectsUser{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(user, "cloudscale-user"))
	assert.Equal(t, "existing-user", user.GetName())

}
