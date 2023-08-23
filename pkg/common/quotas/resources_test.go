package quotas

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
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
