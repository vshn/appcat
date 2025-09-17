package webhooks

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

func TestObjectbucketDeletionProtectionHandler_ValidateDelete(t *testing.T) {

	bucket := appcatv1.ObjectBucket{
		Spec: appcatv1.ObjectBucketSpec{
			Parameters: appcatv1.ObjectBucketParameters{
				Security: vshnv1.Security{
					DeletionProtection: true,
				},
			},
		},
	}

	p := &ObjectbucketDeletionProtectionHandler{
		log: logr.Discard(),
	}
	_, err := p.ValidateDelete(context.TODO(), &bucket)

	assert.Error(t, err)

	bucket.Spec.Parameters.Security.DeletionProtection = false

	_, err = p.ValidateDelete(context.TODO(), &bucket)

	assert.NoError(t, err)

}

func TestObjectbucketDeletionProtectionHandler_ValidateUpdate_BucketNameChange(t *testing.T) {
	oldBucket := &appcatv1.ObjectBucket{
		Spec: appcatv1.ObjectBucketSpec{
			Parameters: appcatv1.ObjectBucketParameters{
				BucketName: "original-name",
			},
		},
	}

	newBucket := &appcatv1.ObjectBucket{
		Spec: appcatv1.ObjectBucketSpec{
			Parameters: appcatv1.ObjectBucketParameters{
				BucketName: "changed-name",
			},
		},
	}

	p := &ObjectbucketDeletionProtectionHandler{
		log: logr.Discard(),
	}

	// Test that changing bucketName is rejected
	_, err := p.ValidateUpdate(context.TODO(), oldBucket, newBucket)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucketName cannot be changed")

	// Test that keeping bucketName the same is allowed
	newBucket.Spec.Parameters.BucketName = "original-name"
	_, err = p.ValidateUpdate(context.TODO(), oldBucket, newBucket)
	assert.NoError(t, err)

	// Test that changing from empty to non-empty is also rejected
	// because empty bucketName gets auto-generated, so it shouldn't change
	oldBucket.Spec.Parameters.BucketName = ""
	newBucket.Spec.Parameters.BucketName = "new-name"
	_, err = p.ValidateUpdate(context.TODO(), oldBucket, newBucket)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucketName cannot be changed")

	// Test that changing from non-empty to empty is also rejected
	oldBucket.Spec.Parameters.BucketName = "original-name"
	newBucket.Spec.Parameters.BucketName = ""
	_, err = p.ValidateUpdate(context.TODO(), oldBucket, newBucket)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucketName cannot be changed")
}
