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

func TestParseAllowedResources(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		rules, err := parseAllowedResources("")
		assert.NoError(t, err)
		assert.Nil(t, rules)
	})
	t.Run("whitespace_only", func(t *testing.T) {
		rules, err := parseAllowedResources("   \n  ")
		assert.NoError(t, err)
		assert.Nil(t, rules)
	})
	t.Run("invalid_json", func(t *testing.T) {
		_, err := parseAllowedResources("not json")
		assert.Error(t, err)
	})
	t.Run("valid", func(t *testing.T) {
		raw := `[{"apiGroups":[""],"kinds":["ConfigMap"]},{"apiGroups":["gateway.networking.k8s.io"],"kinds":["ReferenceGrant"]}]`
		rules, err := parseAllowedResources(raw)
		assert.NoError(t, err)
		assert.Equal(t, []AllowedResourceRule{
			{APIGroups: []string{""}, Kinds: []string{"ConfigMap"}},
			{APIGroups: []string{"gateway.networking.k8s.io"}, Kinds: []string{"ReferenceGrant"}},
		}, rules)
	})
}

func TestValidateAdditionalResource(t *testing.T) {
	rules := []AllowedResourceRule{
		{APIGroups: []string{""}, Kinds: []string{"ConfigMap", "Secret"}},
		{APIGroups: []string{"apps"}, Kinds: []string{"Deployment"}},
		{APIGroups: []string{"gateway.networking.k8s.io"}, Kinds: []string{"ReferenceGrant"}},
	}
	cases := []struct {
		name       string
		apiVersion string
		kind       string
		wantErr    bool
	}{
		{"core_configmap_allowed", "v1", "ConfigMap", false},
		{"core_secret_allowed", "v1", "Secret", false},
		{"apps_deployment_allowed", "apps/v1", "Deployment", false},
		{"reference_grant_allowed", "gateway.networking.k8s.io/v1beta1", "ReferenceGrant", false},
		{"core_pod_not_listed", "v1", "Pod", true},
		{"apps_kind_in_wrong_group", "batch/v1", "Deployment", true},
		{"rbac_role_not_listed", "rbac.authorization.k8s.io/v1", "Role", true},
		{"apps_statefulset_not_listed", "apps/v1", "StatefulSet", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := &unstructured.Unstructured{}
			u.SetAPIVersion(tc.apiVersion)
			u.SetKind(tc.kind)
			err := validateAdditionalResource(u, rules)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAdditionalResource_Wildcards(t *testing.T) {
	t.Run("wildcard_group", func(t *testing.T) {
		rules := []AllowedResourceRule{{APIGroups: []string{"*"}, Kinds: []string{"NetworkPolicy"}}}
		u := &unstructured.Unstructured{}
		u.SetAPIVersion("networking.k8s.io/v1")
		u.SetKind("NetworkPolicy")
		assert.NoError(t, validateAdditionalResource(u, rules))
	})
	t.Run("wildcard_kind", func(t *testing.T) {
		rules := []AllowedResourceRule{{APIGroups: []string{"apps"}, Kinds: []string{"*"}}}
		u := &unstructured.Unstructured{}
		u.SetAPIVersion("apps/v1")
		u.SetKind("StatefulSet")
		assert.NoError(t, validateAdditionalResource(u, rules))
	})
}

// Allowlist empty: function is a no-op even with a ref set.
func TestAddAdditionalResources_NoAllowlist(t *testing.T) {
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
