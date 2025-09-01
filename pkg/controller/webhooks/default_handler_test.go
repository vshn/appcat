package webhooks

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	appcatruntime "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func TestDefaultWebhookHandler_ValidateProviderConfig(t *testing.T) {
	ctx := context.TODO()

	tests := []struct {
		name        string
		claimLabels map[string]string
		objects     []client.Object
		expectErrs  int
		expectMsgs  []string
	}{
		{
			name:        "no provider config label",
			claimLabels: map[string]string{},
			objects:     []client.Object{},
			expectErrs:  0,
		},
		{
			name: "empty provider config label",
			claimLabels: map[string]string{
				appcatruntime.ProviderConfigLabel: "",
			},
			objects:    []client.Object{},
			expectErrs: 0,
		},
		{
			name: "missing kubernetes provider config",
			claimLabels: map[string]string{
				appcatruntime.ProviderConfigLabel: "test-config",
			},
			objects:    []client.Object{},
			expectErrs: 2,
			expectMsgs: []string{
				"kubernetes ProviderConfig \"test-config\" not found",
				"helm ProviderConfig \"test-config\" not found",
			},
		},
		{
			name: "valid provider configs with secrets",
			claimLabels: map[string]string{
				appcatruntime.ProviderConfigLabel: "test-config",
			},
			objects: []client.Object{
				createTestKubernetesProviderConfig("test-config", "test-secret", "crossplane-system"),
				createTestHelmProviderConfig("test-config", "test-secret", "crossplane-system"),
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "crossplane-system",
					},
				},
			},
			expectErrs: 0,
		},
		{
			name: "provider configs exist but missing secret",
			claimLabels: map[string]string{
				appcatruntime.ProviderConfigLabel: "test-config",
			},
			objects: []client.Object{
				createTestKubernetesProviderConfig("test-config", "missing-secret", "crossplane-system"),
				createTestHelmProviderConfig("test-config", "missing-secret", "crossplane-system"),
			},
			expectErrs: 2,
			expectMsgs: []string{
				"kubernetes ProviderConfig \"test-config\" references secret crossplane-system/missing-secret which does not exist",
				"helm ProviderConfig \"test-config\" references secret crossplane-system/missing-secret which does not exist",
			},
		},
		{
			name: "provider configs with no credentials",
			claimLabels: map[string]string{
				appcatruntime.ProviderConfigLabel: "test-config",
			},
			objects: []client.Object{
				createTestProviderConfigNoCredentials("kubernetes.crossplane.io", "v1alpha1", "test-config"),
				createTestProviderConfigNoCredentials("helm.crossplane.io", "v1beta1", "test-config"),
			},
			expectErrs: 2,
			expectMsgs: []string{
				"kubernetes ProviderConfig \"test-config\" has no credentials configured",
				"helm ProviderConfig \"test-config\" has no credentials configured",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fclient := fake.NewClientBuilder().
				WithScheme(pkg.SetupScheme()).
				WithObjects(tt.objects...).
				Build()

			handler := &DefaultWebhookHandler{
				client: fclient,
				log:    logr.Discard(),
			}

			claim := &vshnv1.VSHNPostgreSQL{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "test-namespace",
					Labels:    tt.claimLabels,
				},
			}

			errs := handler.ValidateProviderConfig(ctx, claim)

			assert.Equal(t, tt.expectErrs, len(errs), "expected %d errors, got %d", tt.expectErrs, len(errs))

			for i, expectedMsg := range tt.expectMsgs {
				if i < len(errs) {
					assert.Contains(t, errs[i].Detail, expectedMsg, "error %d should contain expected message", i)
				}
			}
		})
	}
}

// Helper functions for creating test objects

func createTestKubernetesProviderConfig(name, secretName, secretNamespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kubernetes.crossplane.io",
		Version: "v1alpha1",
		Kind:    "ProviderConfig",
	})
	obj.SetName(name)

	spec := map[string]interface{}{
		"credentials": map[string]interface{}{
			"secretRef": map[string]interface{}{
				"name":      secretName,
				"namespace": secretNamespace,
				"key":       "kubeconfig",
			},
		},
	}
	obj.Object["spec"] = spec

	return obj
}

func createTestHelmProviderConfig(name, secretName, secretNamespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "helm.crossplane.io",
		Version: "v1beta1",
		Kind:    "ProviderConfig",
	})
	obj.SetName(name)

	spec := map[string]interface{}{
		"credentials": map[string]interface{}{
			"secretRef": map[string]interface{}{
				"name":      secretName,
				"namespace": secretNamespace,
				"key":       "kubeconfig",
			},
		},
	}
	obj.Object["spec"] = spec

	return obj
}

func createTestProviderConfigNoCredentials(group, version, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    "ProviderConfig",
	})
	obj.SetName(name)

	obj.Object["spec"] = map[string]interface{}{}

	return obj
}
