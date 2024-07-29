package cloudscalebucket

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
)

func TestProvisionMiniobucket(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "miniobucket/bucket.yaml")

	ctx := context.TODO()

	bucketName := "mytest"

	res := ProvisionCloudscalebucket(ctx, svc)
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
