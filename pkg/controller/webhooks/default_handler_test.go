package webhooks

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSetupWebhookHandlerWithManager_ValidateCreate(t *testing.T) {
	// Given
	claimNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "claimns",
			Labels: map[string]string{
				utils.OrgLabelName: "myorg",
			},
		},
	}

	ctx := context.TODO()

	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(claimNS).
		Build()

	handler := TestWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:    fclient,
			log:       logr.Discard(),
			withQuota: true,
			obj:       &vshnv1.VSHNNextcloud{},
			name:      "nextcloud",
		},
	}

	redisOrig := &vshnv1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNNextcloudSpec{
			Parameters: vshnv1.VSHNNextcloudParameters{
				Size: vshnv1.VSHNSizeSpec{
					Requests: vshnv1.VSHNDBaaSSizeRequestsSpec{
						CPU: "500m",
					},
				},
			},
		},
	}

	// When within quota
	_, err := handler.ValidateCreate(ctx, redisOrig)

	//Then no err
	assert.NoError(t, err)

	// When quota breached
	// CPU Requests
	nextcloudInvalid := redisOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Size.Requests.CPU = "5000m"
	_, err = handler.ValidateCreate(ctx, nextcloudInvalid)
	assert.Error(t, err)

	// CPU Limit
	nextcloudInvalid = redisOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Size.CPU = "5000m"
	_, err = handler.ValidateCreate(ctx, nextcloudInvalid)
	assert.Error(t, err)

	// Memory Limit
	nextcloudInvalid = redisOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Size.Memory = "25Gi"
	_, err = handler.ValidateCreate(ctx, nextcloudInvalid)
	assert.Error(t, err)

	// Memory Requests
	nextcloudInvalid = redisOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Size.Requests.Memory = "25Gi"
	_, err = handler.ValidateCreate(ctx, nextcloudInvalid)
	assert.Error(t, err)

	// Disk
	nextcloudInvalid = redisOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Size.Disk = "25Ti"
	_, err = handler.ValidateCreate(ctx, nextcloudInvalid)
	assert.Error(t, err)

	//When invalid size
	// CPU Requests
	nextcloudInvalid = redisOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Size.Requests.CPU = "foo"
	_, err = handler.ValidateCreate(ctx, nextcloudInvalid)
	assert.Error(t, err)

	// CPU Limit
	nextcloudInvalid = redisOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Size.CPU = "foo"
	_, err = handler.ValidateCreate(ctx, nextcloudInvalid)
	assert.Error(t, err)

	// Memory Limit
	nextcloudInvalid = redisOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Size.Memory = "foo"
	_, err = handler.ValidateCreate(ctx, nextcloudInvalid)
	assert.Error(t, err)

	// Memory Requests
	nextcloudInvalid = redisOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Size.Requests.Memory = "foo"
	_, err = handler.ValidateCreate(ctx, nextcloudInvalid)
	assert.Error(t, err)

	// Disk
	nextcloudInvalid = redisOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Size.Disk = "foo"
	_, err = handler.ValidateCreate(ctx, nextcloudInvalid)
	assert.Error(t, err)

}

func TestSetupWebhookHandlerWithManager_ValidateDelete(t *testing.T) {
	// Given
	claimNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "claimns",
			Labels: map[string]string{
				utils.OrgLabelName: "myorg",
			},
		},
	}

	ctx := context.TODO()

	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(claimNS).
		Build()

	handler := TestWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:    fclient,
			log:       logr.Discard(),
			withQuota: true,
			obj:       &vshnv1.VSHNNextcloud{},
			name:      "nextcloud",
		},
	}

	nextcloudOrig := &vshnv1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNNextcloudSpec{
			Parameters: vshnv1.VSHNNextcloudParameters{
				Security: vshnv1.Security{
					DeletionProtection: true,
				},
			},
		},
	}

	// When within quota
	_, err := handler.ValidateDelete(ctx, nextcloudOrig)

	//Then err
	assert.Error(t, err)

	//Instances
	nextcloudDeletable := nextcloudOrig.DeepCopy()
	nextcloudDeletable.Spec.Parameters.Security.DeletionProtection = false

	_, err = handler.ValidateDelete(ctx, nextcloudDeletable)

	//Then no err
	assert.NoError(t, err)
}
