package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddNamespaceQuotas(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/quotas/01_default.yaml")
	ctx := context.TODO()

	res := addInitialNamespaceQuotas(ctx, svc, "namespace")
	assert.Nil(t, res)

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetDesiredKubeObject(ns, "namespace"))
	assert.NotEmpty(t, ns.GetAnnotations())
}

// TestAddNamespaceQuotas_ExistingQuotas ensures that the quota annotations are written to
// the desired namespace, even if they are already present on the observed namespace. Otherwise
// provider-kubernetes would prune them again with its next server-side apply.
func TestAddNamespaceQuotas_ExistingQuotas(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/quotas/02_existing_quotas.yaml")
	ctx := context.TODO()

	assert.Nil(t, addInitialNamespaceQuotas(ctx, svc, "namespace"))

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetDesiredKubeObject(ns, "namespace"))

	annotations := ns.GetAnnotations()
	for _, annotation := range utils.QuotaAnnotations {
		if annotation == utils.StorageClassesAnnotation {
			// only set on exoscale
			continue
		}
		assert.Contains(t, annotations, annotation)
	}

	// existing values must not be overwritten by the defaults
	assert.Equal(t, "10", annotations[utils.CpuLimitAnnotation])
	assert.Equal(t, "44Gi", annotations[utils.MemoryLimitAnnotation])

	// annotations that aren't managed by us should not be carried over from the observed namespace
	assert.NotContains(t, annotations, "openshift.io/sa.scc.mcs")
}

func TestAddAllowedLabels(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/allowed_labels/01_allowed_labels.yaml")

	comp := &vshnv1.VSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"example.com/allowed":     "allowed-value",
				"example.com/empty":       "",
				"example.com/not-allowed": "secret-value",
			},
		},
	}

	tests := map[string]struct {
		allowedLabelsConfig string
		nsLabels            map[string]string
		expected            map[string]string
	}{
		"GivenAllowedLabelOnComposite_ThenCopiedToNamespace": {
			allowedLabelsConfig: `["example.com/allowed"]`,
			nsLabels:            map[string]string{},
			expected:            map[string]string{"example.com/allowed": "allowed-value"},
		},
		"GivenLabelNotInAllowedLabels_ThenNotCopied": {
			allowedLabelsConfig: `["example.com/other"]`,
			nsLabels:            map[string]string{},
			expected:            map[string]string{},
		},
		"GivenAllowedLabelIsEmpty_ThenNotCopied": {
			allowedLabelsConfig: `["example.com/empty"]`,
			nsLabels:            map[string]string{},
			expected:            map[string]string{},
		},
		"GivenAllowedLabelMissingOnComposite_ThenNotCopied": {
			allowedLabelsConfig: `["example.com/missing"]`,
			nsLabels:            map[string]string{},
			expected:            map[string]string{},
		},
		"GivenNoAllowedLabelsConfigured_ThenNamespaceLabelsUnchanged": {
			allowedLabelsConfig: "",
			nsLabels:            map[string]string{"appcat.vshn.io/servicename": "postgresql-standalone"},
			expected:            map[string]string{"appcat.vshn.io/servicename": "postgresql-standalone"},
		},
		"GivenInvalidAllowedLabelsConfig_ThenNamespaceLabelsUnchanged": {
			allowedLabelsConfig: `this is not json`,
			nsLabels:            map[string]string{},
			expected:            map[string]string{},
		},
		"GivenExistingNamespaceLabel_ThenAllowedLabelOverridesIt": {
			allowedLabelsConfig: `["example.com/allowed"]`,
			nsLabels:            map[string]string{"example.com/allowed": "old-value"},
			expected:            map[string]string{"example.com/allowed": "allowed-value"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc.Config.Data["allowedNamespaceLabels"] = tc.allowedLabelsConfig

			addAllowedLabels(tc.nsLabels, comp, svc)

			assert.Equal(t, tc.expected, tc.nsLabels)
			assert.NotContains(t, tc.nsLabels, "example.com/not-allowed")
		})
	}
}

func TestCreateInstanceNamespace_AllowedLabels(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/allowed_labels/01_allowed_labels.yaml")
	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	err := createInstanceNamespace("postgresql", "pgsql-gc9x4", "unit-test", "my-psql", "namespace", "pgsql", "billing", svc, comp)
	assert.NoError(t, err)

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetDesiredKubeObject(ns, "namespace"))

	labels := ns.GetLabels()
	assert.Equal(t, "allowed-value", labels["example.com/allowed"])
	assert.NotContains(t, labels, "example.com/not-allowed")
	assert.NotContains(t, labels, "example.com/empty")
	assert.Equal(t, "postgresql-standalone", labels["appcat.vshn.io/servicename"])
}

// A malformed config must not fail the reconciliation, it only results in a warning
// and no labels are copied to the namespace.
func TestCreateInstanceNamespace_InvalidAllowedLabels(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "common/allowed_labels/02_invalid_allowed_labels.yaml")
	comp := &vshnv1.VSHNPostgreSQL{}
	assert.NoError(t, svc.GetObservedComposite(comp))

	err := createInstanceNamespace("postgresql", "pgsql-gc9x4", "unit-test", "my-psql", "namespace", "pgsql", "billing", svc, comp)
	assert.NoError(t, err)

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetDesiredKubeObject(ns, "namespace"))
	assert.NotContains(t, ns.GetLabels(), "example.com/allowed")
}
