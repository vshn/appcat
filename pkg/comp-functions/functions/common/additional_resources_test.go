package common

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestStripYAMLExtension(t *testing.T) {
	tests := map[string]string{
		"cm.yaml":     "cm",
		"cm.yml":      "cm",
		"cm.json":     "cm",
		"my-resource": "my-resource",
		"my.svc.yaml": "my.svc",
		"":            "",
	}
	for in, want := range tests {
		assert.Equal(t, want, stripYAMLExtension(in), "input: %q", in)
	}
}

func TestValidateAdditionalResource(t *testing.T) {
	cases := []struct {
		name       string
		apiVersion string
		wantErr    bool
	}{
		{"core_v1_allowed", "v1", false},
		{"apps_v1_allowed", "apps/v1", false},
		{"batch_allowed", "batch/v1", false},
		{"networking_allowed", "networking.k8s.io/v1", false},
		{"rbac_blocked", "rbac.authorization.k8s.io/v1", true},
		{"admission_blocked", "admissionregistration.k8s.io/v1", true},
		{"crd_blocked", "apiextensions.k8s.io/v1", true},
		{"authz_blocked", "authorization.k8s.io/v1", true},
		{"certs_blocked", "certificates.k8s.io/v1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := &unstructured.Unstructured{}
			u.SetAPIVersion(tc.apiVersion)
			err := validateAdditionalResource(u)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Feature flag off: function is a no-op even with a ref set.
func TestAddAdditionalResources_FeatureFlagDisabled(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/additional_resources/01-FeatureDisabled.yaml")

	res := AddAdditionalResources[*vshnv1.VSHNPostgreSQL](context.Background(), &vshnv1.VSHNPostgreSQL{}, svc)
	assert.Nil(t, res)
	assert.Empty(t, svc.GetAllDesired())
}

// Feature flag on but no ConfigMapRef: no-op.
func TestAddAdditionalResources_NoConfigMapRef(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/additional_resources/02-NoConfigMapRef.yaml")

	res := AddAdditionalResources[*vshnv1.VSHNPostgreSQL](context.Background(), &vshnv1.VSHNPostgreSQL{}, svc)
	assert.Nil(t, res)
	// The observer ConfigMap should not be set
	for name := range svc.GetAllDesired() {
		assert.NotContains(t, string(name), "additional-resources-cm")
	}
}

// ConfigMapRef set, observed ConfigMap not yet present: observer is set up but no resources deployed yet.
func TestAddAdditionalResources_ObserverNotReady(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/additional_resources/03-ObserverNotReady.yaml")

	res := AddAdditionalResources[*vshnv1.VSHNPostgreSQL](context.Background(), &vshnv1.VSHNPostgreSQL{}, svc)
	assert.Nil(t, res)

	// The observer ConfigMap should be desired
	observerName := "psql-additional-resources-cm"
	obs := &xkube.Object{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(obs, observerName))

	// And no resource named with the "additional-" prefix should exist yet
	for name := range svc.GetAllDesired() {
		assert.NotContains(t, string(name), "-additional-test")
	}
}

// ConfigMap observed with one allowed and one blocked entry: allowed is deployed, blocked is skipped with a warning.
func TestAddAdditionalResources_AllowedAndBlocked(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/additional_resources/04-AllowedAndBlocked.yaml")

	res := AddAdditionalResources[*vshnv1.VSHNPostgreSQL](context.Background(), &vshnv1.VSHNPostgreSQL{}, svc)
	assert.Nil(t, res)

	// Allowed resource must be in desired state
	allowed := &xkube.Object{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(allowed, "psql-additional-test"))

	cm := &corev1.ConfigMap{}
	assert.NoError(t, json.Unmarshal(allowed.Spec.ForProvider.Manifest.Raw, cm))
	assert.Equal(t, "my-test-config", cm.GetName())
	// Namespace must be forced to the instance namespace
	assert.Equal(t, "vshn-postgresql-psql", cm.GetNamespace())

	// Blocked resource must NOT be in desired state
	blocked := &xkube.Object{}
	err := svc.GetDesiredComposedResourceByName(blocked, "psql-additional-bad-role")
	assert.Error(t, err)
}
