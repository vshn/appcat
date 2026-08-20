package utils

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResources_MultiplyBy(t *testing.T) {
	r := Resources{
		CPURequests:    *resource.NewMilliQuantity(400, resource.DecimalSI),
		CPULimits:      *resource.NewMilliQuantity(400, resource.DecimalSI),
		Disk:           *resource.NewQuantity(21474836480, resource.BinarySI),
		MemoryRequests: *resource.NewQuantity(1811939328, resource.BinarySI),
		MemoryLimits:   *resource.NewQuantity(1811939328, resource.BinarySI),
	}

	// Normal multiplication
	r.MultiplyBy(3)
	assert.Equal(t, int64(1200), r.CPULimits.MilliValue())
	assert.Equal(t, int64(1200), r.CPURequests.MilliValue())
	assert.Equal(t, int64(1811939328*3), r.MemoryLimits.Value())
	assert.Equal(t, int64(1811939328*3), r.MemoryRequests.Value())
	assert.Equal(t, int64(21474836480*3), r.Disk.Value())

	// Special case, with no change
	r.MultiplyBy(0)
	assert.Equal(t, int64(1200), r.CPULimits.MilliValue())
	assert.Equal(t, int64(1200), r.CPURequests.MilliValue())
	assert.Equal(t, int64(1811939328*3), r.MemoryLimits.Value())
	assert.Equal(t, int64(1811939328*3), r.MemoryRequests.Value())
	assert.Equal(t, int64(21474836480*3), r.Disk.Value())
}

// The instance namespace quota annotations written by the composition include the sidecar
// overhead. If they can't be read, the fallback defaults have to include it as well,
// otherwise an instance that was allowed on create gets denied on every update.
func TestCheckResourcesAgainstQuotas_NamespaceWithoutAnnotations(t *testing.T) {
	ctx := context.TODO()
	viper.Set("PLANS_NAMESPACE", "testns")

	sidecars := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "vshnpostgresqlplans", Namespace: "testns"},
		Data: map[string]string{
			"sideCars": `{"envoy": {"limits": {"cpu": "4800m", "memory": "6Gi"}, "requests": {"cpu": "32m", "memory": "64Mi"}}}`,
		},
	}
	instanceNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "vshn-postgresql-test"}}

	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(sidecars, instanceNS).Build()
	gk := schema.GroupKind{Group: "vshn.appcat.vshn.io", Kind: "VSHNPostgreSQL"}

	// plan (500m) + sidecars (4800m), which is above the bare default of 5000m but well
	// below the default including sidecars (5000m + 2*4800m).
	r := Resources{
		CPULimits:      *resource.NewMilliQuantity(5300, resource.DecimalSI),
		CPURequests:    *resource.NewMilliQuantity(282, resource.DecimalSI),
		MemoryLimits:   *resource.NewQuantity(7516192768, resource.BinarySI),
		MemoryRequests: *resource.NewQuantity(1140850688, resource.BinarySI),
		Disk:           *resource.NewQuantity(21474836480, resource.BinarySI),
	}

	assert.Nil(t, r.CheckResourcesAgainstQuotas(ctx, c, "test", "vshn-postgresql-test", gk, 1))

	// and it still rejects what's actually over the quota
	r.CPULimits = *resource.NewMilliQuantity(20000, resource.DecimalSI)
	err := r.CheckResourcesAgainstQuotas(ctx, c, "test", "vshn-postgresql-test", gk, 1)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Max allowed CPU limits: 14600m")
}
