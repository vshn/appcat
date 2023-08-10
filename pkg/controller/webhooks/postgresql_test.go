package webhooks

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPostgreSQLWebhookHandler_ValidateCreate(t *testing.T) {
	// Given
	claimNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "claimns",
			Labels: map[string]string{
				"appuio.io/organization": "myorg",
			},
		},
	}

	ctx := context.TODO()

	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(claimNS).
		Build()

	handler := PostgreSQLWebhookHandler{
		client:    fclient,
		log:       logr.Discard(),
		withQuota: true,
	}

	pgOrig := &vshnv1.VSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Size: vshnv1.VSHNDBaaSSizeSpec{
					CPU:    "500m",
					Memory: "1Gi",
				},
			},
		},
	}

	// When within quota
	err := handler.ValidateCreate(ctx, pgOrig)

	//Then no err
	assert.NoError(t, err)

	//When quota breached
	// CPU Limits
	pgInvalid := pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.CPU = "5000m"
	assert.Error(t, handler.ValidateCreate(ctx, pgInvalid))

	//CPU Requests
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Requests.CPU = "5000m"
	assert.Error(t, handler.ValidateCreate(ctx, pgInvalid))

	//Memory Limits
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Memory = "25Gi"
	assert.Error(t, handler.ValidateCreate(ctx, pgInvalid))

	//Memory requests
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Requests.Memory = "25Gi"
	assert.Error(t, handler.ValidateCreate(ctx, pgInvalid))

	//Disk
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Disk = "25Ti"
	assert.Error(t, handler.ValidateCreate(ctx, pgInvalid))

	//When invalid size
	// CPU Limits
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.CPU = "foo"
	assert.Error(t, handler.ValidateCreate(ctx, pgInvalid))

	//CPU Requests
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Requests.CPU = "foo"
	assert.Error(t, handler.ValidateCreate(ctx, pgInvalid))

	//Memory Limits
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Memory = "foo"
	assert.Error(t, handler.ValidateCreate(ctx, pgInvalid))

	//Memory requests
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Requests.Memory = "foo"
	assert.Error(t, handler.ValidateCreate(ctx, pgInvalid))

	//Disk
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Disk = "foo"
	assert.Error(t, handler.ValidateCreate(ctx, pgInvalid))
}
