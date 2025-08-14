package webhooks

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	assert.NoError(t, validateCustomFilePaths(
		[]vshnv1.VSHNKeycloakCustomFile{
			{
				Source:      "blacklist.txt",
				Destination: "data/password-blacklists/blacklist.txt",
			},
		},
	))
}

func TestKeycloakWebhookHandler_ValidatePostgreSQLEncryptionChanges(t *testing.T) {
	ctx := context.TODO()
	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		Build()

	handler := KeycloakWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:    fclient,
			log:       logr.Discard(),
			withQuota: false,
			obj:       &vshnv1.VSHNKeycloak{},
			name:      "keycloak",
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
