package spksredis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	spksv1alpha1 "github.com/vshn/appcat/v4/apis/syntools/v1alpha1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_scaleRedisRelease(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "spksredis/01-scaleRedis.yaml")
	comp := &spksv1alpha1.CompositeRedisInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CompositeRedisInstance",
			APIVersion: "syn.tools/v1alpha1",
		},
	}

	// Scale down and trigger new reconcile via status update
	assert.NoError(t, scaleRedisRelease(svc, comp))
	assert.Equal(t, 2, len(svc.GetAllDesired()))
	assert.NoError(t, svc.GetDesiredComposite(comp))
	assert.NotEmpty(t, comp.Status.ScaleTimeStamp)

	svc = commontest.LoadRuntimeFromFile(t, "spksredis/02-scaleRedisFinished.yaml")
	comp = &spksv1alpha1.CompositeRedisInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CompositeRedisInstance",
			APIVersion: "syn.tools/v1alpha1",
		},
	}

	// Scaling finished, no more status update expected.
	// No more sts observer expected
	assert.NoError(t, scaleRedisRelease(svc, comp))
	assert.Equal(t, 1, len(svc.GetAllDesired()))
	assert.NoError(t, svc.GetDesiredComposite(comp))
	assert.Empty(t, comp.Status.ScaleTimeStamp)

}
