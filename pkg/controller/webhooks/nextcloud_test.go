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
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  true,
			obj:        &vshnv1.VSHNNextcloud{},
			name:       "nextcloud",
			nameLength: 30,
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

	// Then no err
	assert.NoError(t, err)

	// When FQDN invalid
	nextcloudInvalid := nextcloudOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Service.FQDN = []string{
		"n€xtcloud.example.tld",
	}

	_, err = handler.ValidateCreate(ctx, nextcloudInvalid)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "FQDN n€xtcloud.example.tld is not a valid DNS name")
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
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  true,
			obj:        &vshnv1.VSHNNextcloud{},
			name:       "nextcloud",
			nameLength: 30,
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

	// Then no err
	assert.NoError(t, err)

	// When FQDN invalid
	nextcloudInvalid := nextcloudOrig.DeepCopy()
	nextcloudInvalid.Spec.Parameters.Service.FQDN = []string{
		"n€xtcloud.example.tld",
	}

	_, err = handler.ValidateUpdate(ctx, nextcloudOrig, nextcloudInvalid)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "FQDN n€xtcloud.example.tld is not a valid DNS name")
}

func TestNextcloudWebhookHandler_ValidateCreate_CollaboraFQDN(t *testing.T) {
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
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  true,
			obj:        &vshnv1.VSHNNextcloud{},
			name:       "nextcloud",
			nameLength: 30,
		},
	}

	// Test 1: Collabora disabled - no FQDN validation should occur
	nextcloudCollaboraDisabled := &vshnv1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNNextcloudSpec{
			Parameters: vshnv1.VSHNNextcloudParameters{
				Service: vshnv1.VSHNNextcloudServiceSpec{
					FQDN: []string{"mynextcloud.example.tld"},
					Collabora: vshnv1.CollaboraSpec{
						Enabled: false,
						FQDN:    "", // Empty FQDN should be fine when disabled
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

	_, err := handler.ValidateCreate(ctx, nextcloudCollaboraDisabled)
	assert.NoError(t, err, "Collabora disabled with empty FQDN should pass validation")

	// Test 2: Collabora enabled with valid FQDN - should pass
	nextcloudCollaboraValid := &vshnv1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNNextcloudSpec{
			Parameters: vshnv1.VSHNNextcloudParameters{
				Service: vshnv1.VSHNNextcloudServiceSpec{
					FQDN: []string{"mynextcloud.example.tld"},
					Collabora: vshnv1.CollaboraSpec{
						Enabled: true,
						FQDN:    "collabora.example.tld",
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

	_, err = handler.ValidateCreate(ctx, nextcloudCollaboraValid)
	assert.NoError(t, err, "Collabora enabled with valid FQDN should pass validation")

	// Test 3: Collabora enabled with invalid FQDN - should fail
	nextcloudCollaboraInvalid := &vshnv1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNNextcloudSpec{
			Parameters: vshnv1.VSHNNextcloudParameters{
				Service: vshnv1.VSHNNextcloudServiceSpec{
					FQDN: []string{"mynextcloud.example.tld"},
					Collabora: vshnv1.CollaboraSpec{
						Enabled: true,
						FQDN:    "c€llabora.example.tld", // Invalid characters
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

	_, err = handler.ValidateCreate(ctx, nextcloudCollaboraInvalid)
	assert.Error(t, err, "Collabora enabled with invalid FQDN should fail validation")
	assert.ErrorContains(t, err, "FQDN c€llabora.example.tld is not a valid DNS name")

	// Test 4: Collabora enabled with empty FQDN - should fail
	nextcloudCollaboraEmpty := &vshnv1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNNextcloudSpec{
			Parameters: vshnv1.VSHNNextcloudParameters{
				Service: vshnv1.VSHNNextcloudServiceSpec{
					FQDN: []string{"mynextcloud.example.tld"},
					Collabora: vshnv1.CollaboraSpec{
						Enabled: true,
						FQDN:    "", // Empty FQDN when enabled
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

	_, err = handler.ValidateCreate(ctx, nextcloudCollaboraEmpty)
	assert.Error(t, err, "Collabora enabled with empty FQDN should fail validation")
	assert.ErrorContains(t, err, "Collabora FQDN is required when Collabora is enabled")
}

func TestNextcloudWebhookHandler_ValidateUpdate_CollaboraFQDN(t *testing.T) {
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
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  true,
			obj:        &vshnv1.VSHNNextcloud{},
			name:       "nextcloud",
			nameLength: 30,
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
					FQDN: []string{"mynextcloud.example.tld"},
					Collabora: vshnv1.CollaboraSpec{
						Enabled: false,
						FQDN:    "",
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

	// Test 1: Update with Collabora disabled - should pass even with empty FQDN
	nextcloudUpdated := nextcloudOrig.DeepCopy()
	nextcloudUpdated.Spec.Parameters.Service.Collabora.Enabled = false
	nextcloudUpdated.Spec.Parameters.Service.Collabora.FQDN = ""

	_, err := handler.ValidateUpdate(ctx, nextcloudOrig, nextcloudUpdated)
	assert.NoError(t, err, "Collabora disabled with empty FQDN should pass validation")

	// Test 2: Enable Collabora with valid FQDN - should pass
	nextcloudEnableValid := nextcloudOrig.DeepCopy()
	nextcloudEnableValid.Spec.Parameters.Service.Collabora.Enabled = true
	nextcloudEnableValid.Spec.Parameters.Service.Collabora.FQDN = "collabora.example.tld"

	_, err = handler.ValidateUpdate(ctx, nextcloudOrig, nextcloudEnableValid)
	assert.NoError(t, err, "Enabling Collabora with valid FQDN should pass validation")

	// Test 3: Enable Collabora with invalid FQDN - should fail
	nextcloudEnableInvalid := nextcloudOrig.DeepCopy()
	nextcloudEnableInvalid.Spec.Parameters.Service.Collabora.Enabled = true
	nextcloudEnableInvalid.Spec.Parameters.Service.Collabora.FQDN = "c€llabora.example.tld"

	_, err = handler.ValidateUpdate(ctx, nextcloudOrig, nextcloudEnableInvalid)
	assert.Error(t, err, "Enabling Collabora with invalid FQDN should fail validation")
	assert.ErrorContains(t, err, "FQDN c€llabora.example.tld is not a valid DNS name")

	// Test 4: Enable Collabora with empty FQDN - should fail
	nextcloudEnableEmpty := nextcloudOrig.DeepCopy()
	nextcloudEnableEmpty.Spec.Parameters.Service.Collabora.Enabled = true
	nextcloudEnableEmpty.Spec.Parameters.Service.Collabora.FQDN = ""

	_, err = handler.ValidateUpdate(ctx, nextcloudOrig, nextcloudEnableEmpty)
	assert.Error(t, err, "Enabling Collabora with empty FQDN should fail validation")
	assert.ErrorContains(t, err, "Collabora FQDN is required when Collabora is enabled")

	// Test 5: Update Collabora FQDN to another valid one - should pass
	nextcloudWithCollabora := &vshnv1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNNextcloudSpec{
			Parameters: vshnv1.VSHNNextcloudParameters{
				Service: vshnv1.VSHNNextcloudServiceSpec{
					FQDN: []string{"mynextcloud.example.tld"},
					Collabora: vshnv1.CollaboraSpec{
						Enabled: true,
						FQDN:    "collabora.example.tld",
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

	nextcloudUpdateFQDN := nextcloudWithCollabora.DeepCopy()
	nextcloudUpdateFQDN.Spec.Parameters.Service.Collabora.FQDN = "new-collabora.example.tld"

	_, err = handler.ValidateUpdate(ctx, nextcloudWithCollabora, nextcloudUpdateFQDN)
	assert.NoError(t, err, "Updating Collabora FQDN to another valid one should pass validation")
}

func TestNextcloudWebhookHandler_ValidateUpdate_VersionDowngrade(t *testing.T) {
	ctx := context.TODO()
	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		Build()

	handler := NextcloudWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  false,
			obj:        &vshnv1.VSHNNextcloud{},
			name:       "nextcloud",
			nameLength: 30,
		},
	}

	newNC := func(version, pinImageTag string) *vshnv1.VSHNNextcloud {
		return &vshnv1.VSHNNextcloud{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myinstance",
				Namespace: "testns",
			},
			Spec: vshnv1.VSHNNextcloudSpec{
				Parameters: vshnv1.VSHNNextcloudParameters{
					Service: vshnv1.VSHNNextcloudServiceSpec{
						FQDN:    []string{"mynextcloud.example.tld"},
						Version: version,
					},
					Maintenance: vshnv1.VSHNDBaaSMaintenanceScheduleSpec{
						PinImageTag: pinImageTag,
					},
				},
			},
		}
	}

	tests := []struct {
		name        string
		oldVersion  string
		newVersion  string
		pinImageTag string
		wantErr     bool
		errContains string
	}{
		// --- happy path ---
		{name: "major upgrade", oldVersion: "30", newVersion: "31", wantErr: false},
		{name: "same major version", oldVersion: "31", newVersion: "31", wantErr: false},
		{name: "major.minor upgrade", oldVersion: "30.0", newVersion: "31.0", wantErr: false},
		{name: "minor upgrade within major", oldVersion: "30.0", newVersion: "30.1", wantErr: false},
		{name: "same version different formats", oldVersion: "30", newVersion: "30.0", wantErr: false},
		{name: "empty old version (first set)", oldVersion: "", newVersion: "31", wantErr: false},
		{name: "both empty versions", oldVersion: "", newVersion: "", wantErr: false},
		{name: "new version cleared", oldVersion: "31", newVersion: "", wantErr: false},
		{name: "unparsable old version", oldVersion: "garbage", newVersion: "30", wantErr: false},

		// --- downgrade blocked ---
		{name: "major downgrade", oldVersion: "31", newVersion: "30", wantErr: true, errContains: "downgrading from"},
		{name: "minor downgrade within major", oldVersion: "30.1", newVersion: "30.0", wantErr: true, errContains: "downgrading from"},
		{name: "patch downgrade", oldVersion: "30.0.1", newVersion: "30.0.0", wantErr: true, errContains: "downgrading from"},

		// --- invalid new version ---
		{name: "unparsable new version", oldVersion: "30", newVersion: "garbage", wantErr: true, errContains: "invalid version"},

		// --- pinImageTag bypasses check ---
		{name: "downgrade allowed with pinImageTag", oldVersion: "31", newVersion: "30", pinImageTag: "nextcloud:30.0.5", wantErr: false},
		{name: "invalid new version allowed with pinImageTag", oldVersion: "30", newVersion: "garbage", pinImageTag: "nextcloud:30.0.5", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler.ValidateUpdate(ctx, newNC(tt.oldVersion, ""), newNC(tt.newVersion, tt.pinImageTag))
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.ErrorContains(t, err, tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNextcloudWebhookHandler_ValidatePostgreSQLEncryptionChanges(t *testing.T) {
	ctx := context.TODO()
	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		Build()

	handler := NextcloudWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  false,
			obj:        &vshnv1.VSHNNextcloud{},
			name:       "nextcloud",
			nameLength: 30,
		},
	}

	// Test 1: Same encryption state should be valid
	nextcloudOrig := &vshnv1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "testns",
		},
		Spec: vshnv1.VSHNNextcloudSpec{
			Parameters: vshnv1.VSHNNextcloudParameters{
				Service: vshnv1.VSHNNextcloudServiceSpec{
					FQDN: []string{"mynextcloud.example.tld"},
					PostgreSQLParameters: &vshnv1.VSHNPostgreSQLParameters{
						Encryption: vshnv1.VSHNPostgreSQLEncryption{
							Enabled: false,
						},
					},
				},
			},
		},
	}

	nextcloudUpdated := nextcloudOrig.DeepCopy()
	// No changes to encryption state

	_, err := handler.ValidateUpdate(ctx, nextcloudOrig, nextcloudUpdated)
	assert.NoError(t, err)

	// Test 2: Enabling encryption after creation should fail
	nextcloudEncryptionEnabled := nextcloudOrig.DeepCopy()
	nextcloudEncryptionEnabled.Spec.Parameters.Service.PostgreSQLParameters.Encryption.Enabled = true

	_, err = handler.ValidateUpdate(ctx, nextcloudOrig, nextcloudEncryptionEnabled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "encryption setting cannot be changed after instance creation")

	// Test 3: Disabling encryption after creation should fail
	nextcloudOrigEncrypted := nextcloudOrig.DeepCopy()
	nextcloudOrigEncrypted.Spec.Parameters.Service.PostgreSQLParameters.Encryption.Enabled = true

	nextcloudEncryptionDisabled := nextcloudOrigEncrypted.DeepCopy()
	nextcloudEncryptionDisabled.Spec.Parameters.Service.PostgreSQLParameters.Encryption.Enabled = false

	_, err = handler.ValidateUpdate(ctx, nextcloudOrigEncrypted, nextcloudEncryptionDisabled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "encryption setting cannot be changed after instance creation")

	// Test 4: Same encryption state (enabled) should be valid
	nextcloudSameEncryption := nextcloudOrigEncrypted.DeepCopy()
	// No changes to encryption state

	_, err = handler.ValidateUpdate(ctx, nextcloudOrigEncrypted, nextcloudSameEncryption)
	assert.NoError(t, err)

	// Test 5: No PostgreSQL parameters should be valid
	nextcloudNoPostgreSQL := &vshnv1.VSHNNextcloud{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "testns",
		},
		Spec: vshnv1.VSHNNextcloudSpec{
			Parameters: vshnv1.VSHNNextcloudParameters{
				Service: vshnv1.VSHNNextcloudServiceSpec{
					FQDN: []string{"mynextcloud.example.tld"},
					// No PostgreSQLParameters
				},
			},
		},
	}

	_, err = handler.ValidateUpdate(ctx, nextcloudNoPostgreSQL, nextcloudNoPostgreSQL)
	assert.NoError(t, err)
}
