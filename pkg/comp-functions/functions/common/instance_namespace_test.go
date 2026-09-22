package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	corev1 "k8s.io/api/core/v1"
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
