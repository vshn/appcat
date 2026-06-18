package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	apixv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_validateCustomFilePaths(t *testing.T) {
	t.Log("Expect error: Empty source")
	assert.Error(t, validateCustomFilePaths(
		[]vshnv1.VSHNKeycloakCustomFile{
			{
				Source:      "",
				Destination: "file",
			},
		},
	))

	t.Log("Expect error: Empty destination")
	assert.Error(t, validateCustomFilePaths(
		[]vshnv1.VSHNKeycloakCustomFile{
			{
				Source:      "file",
				Destination: "",
			},
		},
	))

	t.Log("Expect error: Root folders")
	for _, folder := range keycloakRootFolders {
		t.Logf("Testing: %s", folder)
		assert.Error(t, validateCustomFilePaths(
			[]vshnv1.VSHNKeycloakCustomFile{
				{
					Source:      "file",
					Destination: fmt.Sprintf("%s/file", folder),
				},
			},
		))
		assert.Error(t, validateCustomFilePaths(
			[]vshnv1.VSHNKeycloakCustomFile{
				{
					Source:      "folder",
					Destination: folder,
				},
			},
		))
	}

	t.Log("Expect error: Path traversal")
	assert.Error(t, validateCustomFilePaths(
		[]vshnv1.VSHNKeycloakCustomFile{
			{
				Source:      "passwd",
				Destination: "../../etc/passwd",
			},
		},
	))

	t.Log("Expect no error: Valid destination")
	assert.Nil(t, validateCustomFilePaths(
		[]vshnv1.VSHNKeycloakCustomFile{
			{
				Source:      "blacklist.txt",
				Destination: "data/password-blacklists/blacklist.txt",
			},
		},
	))
}

func TestValidateCustomImageMutualExclusion(t *testing.T) {
	t.Log("Both customImage and customizationImage set: expect error")
	keycloak := &vshnv1.VSHNKeycloak{
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Service: vshnv1.VSHNKeycloakServiceSpec{
					CustomImage: vshnv1.VSHNKeycloakImage{
						Image: "ghcr.io/my-org/my-keycloak:26.6.1",
					},
					CustomizationImage: vshnv1.VSHNKeycloakCustomizationImage{
						Image: "registry/user/image:tag",
					},
				},
			},
		},
	}
	assert.Error(t, validateCustomImageMutualExclusion(keycloak))

	t.Log("Only customImage set: expect no error")
	keycloakCustomOnly := &vshnv1.VSHNKeycloak{
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Service: vshnv1.VSHNKeycloakServiceSpec{
					CustomImage: vshnv1.VSHNKeycloakImage{
						Image: "ghcr.io/my-org/my-keycloak:26.6.1",
					},
				},
			},
		},
	}
	assert.Nil(t, validateCustomImageMutualExclusion(keycloakCustomOnly))

	t.Log("Only customizationImage set: expect no error")
	keycloakCustomizationOnly := &vshnv1.VSHNKeycloak{
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Service: vshnv1.VSHNKeycloakServiceSpec{
					CustomizationImage: vshnv1.VSHNKeycloakCustomizationImage{
						Image: "registry/user/image:tag",
					},
				},
			},
		},
	}
	assert.Nil(t, validateCustomImageMutualExclusion(keycloakCustomizationOnly))

	t.Log("Neither set: expect no error")
	assert.Nil(t, validateCustomImageMutualExclusion(&vshnv1.VSHNKeycloak{}))
}

func TestWarnPinImageTagIgnoredForCustomImage(t *testing.T) {
	t.Log("Both customImage and pinImageTag set: expect warning")
	keycloak := &vshnv1.VSHNKeycloak{
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Service: vshnv1.VSHNKeycloakServiceSpec{
					CustomImage: vshnv1.VSHNKeycloakImage{
						Image: "ghcr.io/my-org/my-keycloak:26.6.1",
					},
				},
				Maintenance: vshnv1.VSHNDBaaSMaintenanceScheduleSpec{
					PinImageTag: "26.6.1",
				},
			},
		},
	}
	warnings := warnPinImageTagIgnoredForCustomImage(keycloak)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "has no effect")
	assert.Contains(t, warnings[0], "customImage takes precedence")

	t.Log("Only pinImageTag set, no customImage: expect no warning")
	keycloakPinOnly := &vshnv1.VSHNKeycloak{
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Maintenance: vshnv1.VSHNDBaaSMaintenanceScheduleSpec{
					PinImageTag: "26.6.1",
				},
			},
		},
	}
	assert.Empty(t, warnPinImageTagIgnoredForCustomImage(keycloakPinOnly))

	t.Log("Only customImage set, no pinImageTag: expect no warning")
	keycloakImageOnly := &vshnv1.VSHNKeycloak{
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Service: vshnv1.VSHNKeycloakServiceSpec{
					CustomImage: vshnv1.VSHNKeycloakImage{
						Image: "ghcr.io/my-org/my-keycloak:26.6.1",
					},
				},
			},
		},
	}
	assert.Empty(t, warnPinImageTagIgnoredForCustomImage(keycloakImageOnly))
}

func TestWarnRelativePathDisablesOptimized(t *testing.T) {
	// revision builds a Keycloak CompositionRevision labelled with the serviceID (so the webhook's
	// List finds it) and carrying the given keycloak_images_optimized value in its function input.
	revision := func(t *testing.T, name string, revNum int64, optimized string) *apixv1.CompositionRevision {
		raw, err := json.Marshal(&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
			Data:     map[string]string{"keycloak_images_optimized": optimized},
		})
		require.NoError(t, err)
		return &apixv1.CompositionRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{keycloakServiceIDLabel: keycloakServiceID},
			},
			Spec: apixv1.CompositionRevisionSpec{
				Revision: revNum,
				Pipeline: []apixv1.PipelineStep{
					{Step: "keycloak-func", Input: &runtime.RawExtension{Raw: raw}},
				},
			},
		}
	}

	handlerWith := func(objs ...client.Object) KeycloakWebhookHandler {
		return KeycloakWebhookHandler{DefaultWebhookHandler: DefaultWebhookHandler{
			client: fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).WithObjects(objs...).Build(),
			log:    logr.Discard(),
		}}
	}

	claim := func(relativePath string) *vshnv1.VSHNKeycloak {
		return &vshnv1.VSHNKeycloak{
			Spec: vshnv1.VSHNKeycloakSpec{
				Parameters: vshnv1.VSHNKeycloakParameters{
					Service: vshnv1.VSHNKeycloakServiceSpec{RelativePath: relativePath},
				},
			},
		}
	}

	ctx := context.TODO()

	t.Log("latest revision optimized=true + non-root relativePath: expect warning")
	h := handlerWith(revision(t, "rev-1", 1, "false"), revision(t, "rev-2", 2, "true"))
	warnings := h.warnRelativePathDisablesOptimized(ctx, claim("/auth"))
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "relativePath")
	assert.Contains(t, warnings[0], "--optimized is omitted")

	t.Log("trailing slash is also non-root: expect warning")
	assert.Len(t, h.warnRelativePathDisablesOptimized(ctx, claim("/auth/")), 1)

	t.Log("latest revision optimized=false: expect no warning (older revision is true)")
	h = handlerWith(revision(t, "rev-1", 1, "true"), revision(t, "rev-2", 2, "false"))
	assert.Empty(t, h.warnRelativePathDisablesOptimized(ctx, claim("/auth")))

	t.Log("root relativePath: expect no warning (no revision lookup)")
	h = handlerWith(revision(t, "rev-1", 1, "true"))
	assert.Empty(t, h.warnRelativePathDisablesOptimized(ctx, claim("/")))

	t.Log("no revisions present: expect no warning, no error")
	h = handlerWith()
	assert.Empty(t, h.warnRelativePathDisablesOptimized(ctx, claim("/auth")))
}

func TestKeycloakWebhookHandler_ValidatePostgreSQLEncryptionChanges(t *testing.T) {
	ctx := context.TODO()
	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		Build()

	handler := KeycloakWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  false,
			obj:        &vshnv1.VSHNKeycloak{},
			name:       "keycloak",
			nameLength: 30,
		},
	}

	// Test 1: Same encryption state should be valid
	keycloakOrig := &vshnv1.VSHNKeycloak{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "testns",
		},
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Service: vshnv1.VSHNKeycloakServiceSpec{
					PostgreSQLParameters: &vshnv1.VSHNPostgreSQLParameters{
						Encryption: vshnv1.VSHNPostgreSQLEncryption{
							Enabled: false,
						},
					},
				},
			},
		},
	}

	keycloakUpdated := keycloakOrig.DeepCopy()
	// No changes to encryption state

	_, err := handler.ValidateUpdate(ctx, keycloakOrig, keycloakUpdated)
	assert.NoError(t, err)

	// Test 2: Enabling encryption after creation should fail
	keycloakEncryptionEnabled := keycloakOrig.DeepCopy()
	keycloakEncryptionEnabled.Spec.Parameters.Service.PostgreSQLParameters.Encryption.Enabled = true

	_, err = handler.ValidateUpdate(ctx, keycloakOrig, keycloakEncryptionEnabled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "encryption setting cannot be changed after instance creation")

	// Test 3: Disabling encryption after creation should fail
	keycloakOrigEncrypted := keycloakOrig.DeepCopy()
	keycloakOrigEncrypted.Spec.Parameters.Service.PostgreSQLParameters.Encryption.Enabled = true

	keycloakEncryptionDisabled := keycloakOrigEncrypted.DeepCopy()
	keycloakEncryptionDisabled.Spec.Parameters.Service.PostgreSQLParameters.Encryption.Enabled = false

	_, err = handler.ValidateUpdate(ctx, keycloakOrigEncrypted, keycloakEncryptionDisabled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "encryption setting cannot be changed after instance creation")

	// Test 4: Same encryption state (enabled) should be valid
	keycloakSameEncryption := keycloakOrigEncrypted.DeepCopy()
	// No changes to encryption state

	_, err = handler.ValidateUpdate(ctx, keycloakOrigEncrypted, keycloakSameEncryption)
	assert.NoError(t, err)

	// Test 5: No PostgreSQL parameters should be valid
	keycloakNoPostgreSQL := &vshnv1.VSHNKeycloak{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "testns",
		},
		Spec: vshnv1.VSHNKeycloakSpec{
			Parameters: vshnv1.VSHNKeycloakParameters{
				Service: vshnv1.VSHNKeycloakServiceSpec{
					// No PostgreSQLParameters
				},
			},
		},
	}

	_, err = handler.ValidateUpdate(ctx, keycloakNoPostgreSQL, keycloakNoPostgreSQL)
	assert.NoError(t, err)
}
