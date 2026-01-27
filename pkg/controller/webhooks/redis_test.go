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

func TestSetupRedisWebhookHandlerWithManager_ValidateCreate(t *testing.T) {
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
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  true,
			obj:        &vshnv1.VSHNRedis{},
			name:       "redis",
			nameLength: 30,
		},
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
	_, err := handler.ValidateCreate(ctx, redisOrig)

	// Then no err
	assert.NoError(t, err)

	// When name too long
	redisInvalid := redisOrig.DeepCopy()
	redisInvalid.Name = "this-redis-instance-name-is-way-too-long-and-should-fail"
	_, err = handler.ValidateCreate(ctx, redisInvalid)
	assert.Error(t, err)

	// When quota breached
	// CPU Requests
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.CPURequests = "5000m"
	_, err = handler.ValidateCreate(ctx, redisInvalid)
	assert.Error(t, err)

	// CPU Limit
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.CPULimits = "5000m"
	_, err = handler.ValidateCreate(ctx, redisInvalid)
	assert.Error(t, err)

	// Memory Limit
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.MemoryLimits = "25Gi"
	_, err = handler.ValidateCreate(ctx, redisInvalid)
	assert.Error(t, err)

	// Memory Requests
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.MemoryRequests = "25Gi"
	_, err = handler.ValidateCreate(ctx, redisInvalid)
	assert.Error(t, err)

	// Disk
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.Disk = "25Ti"
	_, err = handler.ValidateCreate(ctx, redisInvalid)
	assert.Error(t, err)

	// When invalid size
	// CPU Requests
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.CPURequests = "foo"
	_, err = handler.ValidateCreate(ctx, redisInvalid)
	assert.Error(t, err)

	// CPU Limit
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.CPULimits = "foo"
	_, err = handler.ValidateCreate(ctx, redisInvalid)
	assert.Error(t, err)

	// Memory Limit
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.MemoryLimits = "foo"
	_, err = handler.ValidateCreate(ctx, redisInvalid)
	assert.Error(t, err)

	// Memory Requests
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.MemoryRequests = "foo"
	_, err = handler.ValidateCreate(ctx, redisInvalid)
	assert.Error(t, err)

	// Disk
	redisInvalid = redisOrig.DeepCopy()
	redisInvalid.Spec.Parameters.Size.Disk = "foo"
	_, err = handler.ValidateCreate(ctx, redisInvalid)
	assert.Error(t, err)
}

func TestSetupRedisWebhookHandlerWithManager_ValidateDelete(t *testing.T) {
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
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  true,
			obj:        &vshnv1.VSHNRedis{},
			name:       "redis",
			nameLength: 30,
		},
	}

	redisOrig := &vshnv1.VSHNRedis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNRedisSpec{
			Parameters: vshnv1.VSHNRedisParameters{
				Security: vshnv1.Security{
					DeletionProtection: true,
				},
			},
		},
	}

	// When within quota
	_, err := handler.ValidateDelete(ctx, redisOrig)

	// Then err
	assert.Error(t, err)

	// Instances
	redisDeletable := redisOrig.DeepCopy()
	redisDeletable.Spec.Parameters.Security.DeletionProtection = false

	_, err = handler.ValidateDelete(ctx, redisDeletable)

	// Then no err
	assert.NoError(t, err)
}
