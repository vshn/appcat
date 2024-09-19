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
