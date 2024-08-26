package vshnpostgres

import (
	"context"
	"testing"
	"time"

	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"

	"github.com/stretchr/testify/assert"
	sgv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	// fnv1aplha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
)

func TestTransformRestart_NoopNoPending(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/restart/01-NoPendingReboot.yaml")

	res := TransformRestart(context.TODO(), &vshnv1.VSHNPostgreSQL{}, svc)
	assert.Nil(t, res)

	comp := &vshnv1.XVSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	desired := svc.GetAllDesired()
	assert.Len(t, desired, 0)
}
func TestTransformRestart_NoopPendingOnRestart(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/restart/02-PendingRebootNoRestart.yaml")

	res := TransformRestart(context.TODO(), &vshnv1.VSHNPostgreSQL{}, svc)
	assert.Nil(t, res)

	comp := &vshnv1.XVSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	desired := svc.GetAllDesired()
	assert.Len(t, desired, 0)
}

func TestTransformRestart_RestartPending(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/restart/02-PendingReboot.yaml")

	res := TransformRestart(context.TODO(), &vshnv1.VSHNPostgreSQL{}, svc)
	assert.Nil(t, res)

	comp := &vshnv1.XVSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	desired := svc.GetAllDesired()
	assert.Len(t, desired, 1)

	sgrest := sgv1.SGDbOps{}
	assert.NoError(t, svc.GetDesiredKubeObject(&sgrest, "pgsql-gc9x4-pg-restart-1682587342"))
	assert.Equal(t, "restart", sgrest.Spec.Op)
	if assert.NotNil(t, sgrest.Spec.RunAt) {
		assert.Equal(t, "2023-04-27T09:22:22Z", *sgrest.Spec.RunAt)
	}
	if assert.NotNil(t, sgrest.Spec.Restart) {
		if assert.NotNil(t, sgrest.Spec.Restart.Method) {
			assert.Equal(t, "InPlace", *sgrest.Spec.Restart.Method)
		}
		if assert.NotNil(t, sgrest.Spec.Restart.OnlyPendingRestart) {
			assert.True(t, *sgrest.Spec.Restart.OnlyPendingRestart)
		}
	}
	assert.Equal(t, "restart", sgrest.Spec.Op)
}

func TestTransformRestart_KeepRecentReboots(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/restart/03-KeepRecentReboots.yaml")

	now := func() time.Time {
		n, err := time.Parse(time.RFC3339, "2023-04-27T10:04:05Z")
		assert.NoError(t, err)
		return n
	}

	res := transformRestart(context.TODO(), svc, now)
	assert.Nil(t, res)

	comp := &vshnv1.XVSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	desired := svc.GetAllDesired()
	assert.Len(t, desired, 2)
}
