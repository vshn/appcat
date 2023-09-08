package miniobucket

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProvisionMiniobucket(t *testing.T) {
	iof := commontest.LoadRuntimeFromFile(t, "miniobucket/bucket.yaml")

	ctx := context.TODO()

	bucketName := "mytest"

	comp := &appcatv1.ObjectBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testbucket",
			Namespace: "default",
		},
		Spec: appcatv1.ObjectBucketSpec{
			Parameters: appcatv1.ObjectBucketParameters{
				BucketName: bucketName,
			},
		},
	}

	assert.NoError(t, iof.Desired.SetComposite(ctx, comp))

	ProvisionMiniobucket(ctx, iof)

	bucket := &miniov1.Bucket{}
	assert.NoError(t, iof.Desired.Get(ctx, bucket, "minio-bucket"))
	assert.Equal(t, bucketName, bucket.GetName())

	user := &miniov1.User{}
	assert.NoError(t, iof.Desired.Get(ctx, user, "minio-user"))
	assert.Equal(t, bucketName, user.GetName())
	assert.Contains(t, user.Spec.ForProvider.Policies, bucketName)

	policy := &miniov1.Policy{}
	assert.NoError(t, iof.Desired.Get(ctx, policy, "minio-policy"))
	assert.Equal(t, bucketName, policy.GetName())

}
