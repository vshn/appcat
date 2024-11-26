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

	handler := KeycloakWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:    fclient,
			log:       logr.Discard(),
			withQuota: true,
			obj:       &vshnv1.VSHNKeycloak{},
			name:      "keycloak",
		},
	}

	keycloakOrig := &vshnv1.VSHNKeycloak{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Size: vshnv1.VSHNSizeSpec{
					Requests: vshnv1.VSHNDBaaSSizeRequestsSpec{
						CPU: "500m",
					},
				},
			},
		},
	}

	// When within quota
	_, err := handler.ValidateCreate(ctx, keycloakOrig)

	//Then no err
	assert.NoError(t, err)

	// When quota breached
	// CPU Requests
	keycloakInvalid := keycloakOrig.DeepCopy()
	keycloakInvalid.Spec.Parameters.Size.Requests.CPU = "5000m"
	_, err = handler.ValidateCreate(ctx, keycloakInvalid)
	assert.Error(t, err)

	// CPU Limit
	keycloakInvalid = keycloakOrig.DeepCopy()
	keycloakInvalid.Spec.Parameters.Size.CPU = "5000m"
	_, err = handler.ValidateCreate(ctx, keycloakInvalid)
	assert.Error(t, err)

	// Memory Limit
	keycloakInvalid = keycloakOrig.DeepCopy()
	keycloakInvalid.Spec.Parameters.Size.Memory = "25Gi"
	_, err = handler.ValidateCreate(ctx, keycloakInvalid)
	assert.Error(t, err)

	// Memory Requests
	keycloakInvalid = keycloakOrig.DeepCopy()
	keycloakInvalid.Spec.Parameters.Size.Requests.Memory = "25Gi"
	_, err = handler.ValidateCreate(ctx, keycloakInvalid)
	assert.Error(t, err)

	// Disk
	keycloakInvalid = keycloakOrig.DeepCopy()
	keycloakInvalid.Spec.Parameters.Size.Disk = "25Ti"
	_, err = handler.ValidateCreate(ctx, keycloakInvalid)
	assert.Error(t, err)

	//When invalid size
	// CPU Requests
	keycloakInvalid = keycloakOrig.DeepCopy()
	keycloakInvalid.Spec.Parameters.Size.Requests.CPU = "foo"
	_, err = handler.ValidateCreate(ctx, keycloakInvalid)
	assert.Error(t, err)

	// CPU Limit
	keycloakInvalid = keycloakOrig.DeepCopy()
	keycloakInvalid.Spec.Parameters.Size.CPU = "foo"
	_, err = handler.ValidateCreate(ctx, keycloakInvalid)
	assert.Error(t, err)

	// Memory Limit
	keycloakInvalid = keycloakOrig.DeepCopy()
	keycloakInvalid.Spec.Parameters.Size.Memory = "foo"
	_, err = handler.ValidateCreate(ctx, keycloakInvalid)
	assert.Error(t, err)

	// Memory Requests
	keycloakInvalid = keycloakOrig.DeepCopy()
	keycloakInvalid.Spec.Parameters.Size.Requests.Memory = "foo"
	_, err = handler.ValidateCreate(ctx, keycloakInvalid)
	assert.Error(t, err)

	// Disk
	keycloakInvalid = keycloakOrig.DeepCopy()
	keycloakInvalid.Spec.Parameters.Size.Disk = "foo"
	_, err = handler.ValidateCreate(ctx, keycloakInvalid)
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

	handler := KeycloakWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:    fclient,
			log:       logr.Discard(),
			withQuota: true,
			obj:       &vshnv1.VSHNKeycloak{},
			name:      "keycloak",
		},
	}

	keycloakOrig := &vshnv1.VSHNKeycloak{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Security: vshnv1.Security{
					DeletionProtection: true,
				},
			},
		},
	}

	// When within quota
	_, err := handler.ValidateDelete(ctx, keycloakOrig)

	//Then err
	assert.Error(t, err)

	//Instances
	keycloakDeletable := keycloakOrig.DeepCopy()
	keycloakDeletable.Spec.Parameters.Security.DeletionProtection = false

	_, err = handler.ValidateDelete(ctx, keycloakDeletable)

	//Then no err
	assert.NoError(t, err)
}
