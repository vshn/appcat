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

func TestSetupRedisWebhookHandlerWithManager(t *testing.T) {
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

	handler := RedisWebhookHandler{
		client:    fclient,
		log:       logr.Discard(),
		withQuota: true,
	}

	redisOrig := &vshnv1.VSHNRedis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNRedisSpec{
			Parameters: vshnv1.VSHNRedisParameters{
				Size: vshnv1.VSHNRedisSizeSpec{
					CPURequests: "500m",
				},
			},
		},
	}

	// When within quota
	err := handler.ValidateCreate(ctx, redisOrig)

	//Then no err
	assert.NoError(t, err)

	//When quota breached
	// CPU Requests
	redisInvalid := redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.CPURequests = "5000m"
	assert.Error(t, handler.ValidateCreate(ctx, redisInvalid))

	// CPU Limit
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.CPULimits = "5000m"
	assert.Error(t, handler.ValidateCreate(ctx, redisInvalid))

	// Memory Limit
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.MemoryLimits = "25Gi"
	assert.Error(t, handler.ValidateCreate(ctx, redisInvalid))

	// Memory Requests
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.MemoryLimits = "25Gi"
	assert.Error(t, handler.ValidateCreate(ctx, redisInvalid))

	// Disk
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.Disk = "25Ti"
	assert.Error(t, handler.ValidateCreate(ctx, redisInvalid))

	//When invalid size
	// CPU Requests
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.CPURequests = "foo"
	assert.Error(t, handler.ValidateCreate(ctx, redisInvalid))

	// CPU Limit
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.CPULimits = "foo"
	assert.Error(t, handler.ValidateCreate(ctx, redisInvalid))

	// Memory Limit
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.MemoryLimits = "foo"
	assert.Error(t, handler.ValidateCreate(ctx, redisInvalid))

	// Memory Requests
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.MemoryLimits = "foo"
	assert.Error(t, handler.ValidateCreate(ctx, redisInvalid))

	// Disk
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.Disk = "foo"
	assert.Error(t, handler.ValidateCreate(ctx, redisInvalid))

}
