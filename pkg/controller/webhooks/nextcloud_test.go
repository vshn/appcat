package webhooks

import (
	"context"
	"fmt"
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

func TestWebhookHandlerWithManager_ValidateCreate_FQDN(t *testing.T) {
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

	handler := NextcloudWebhookHandler{
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
				Service: vshnv1.VSHNNextcloudServiceSpec{
					FQDN: []string{
						"mynextcloud.example.tld",
					},
				},
				Size: vshnv1.VSHNSizeSpec{
					Requests: vshnv1.VSHNDBaaSSizeRequestsSpec{
						CPU: "500m",
					},
				},
			},
		},
	}

	_, err := handler.ValidateCreate(ctx, nextcloudOrig)

	//Then no err
	assert.NoError(t, err)

	// When FQDN invalid
	nextcloudInvalid := nextcloudOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Service.FQDN = []string{
		"n€xtcloud.example.tld",
	}

	_, err = handler.ValidateCreate(ctx, nextcloudInvalid)
	assert.Error(t, err)
	assert.Equal(t, fmt.Errorf("FQDN n€xtcloud.example.tld is not a valid DNS name"), err)
}

func TestWebhookHandlerWithManager_ValidateUpdate_FQDN(t *testing.T) {
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

	handler := NextcloudWebhookHandler{
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
				Service: vshnv1.VSHNNextcloudServiceSpec{
					FQDN: []string{
						"mynextcloud.example.tld",
					},
				},
				Size: vshnv1.VSHNSizeSpec{
					Requests: vshnv1.VSHNDBaaSSizeRequestsSpec{
						CPU: "500m",
					},
				},
			},
		},
	}
	nextcloudNew := nextcloudOrig.DeepCopy()
	nextcloudNew.Spec.Parameters.Service.FQDN = append(nextcloudNew.Spec.Parameters.Service.FQDN, "myother-nexctloud.example.tld")

	_, err := handler.ValidateUpdate(ctx, nextcloudOrig, nextcloudNew)

	//Then no err
	assert.NoError(t, err)

	// When FQDN invalid
	nextcloudInvalid := nextcloudOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Service.FQDN = []string{
		"n€xtcloud.example.tld",
	}

	_, err = handler.ValidateUpdate(ctx, nextcloudOrig, nextcloudInvalid)
	assert.Error(t, err)
	assert.Equal(t, fmt.Errorf("FQDN n€xtcloud.example.tld is not a valid DNS name"), err)
}
