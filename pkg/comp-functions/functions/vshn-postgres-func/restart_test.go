package vshnpostgres

import (
	"context"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sgv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"

	fnv1aplha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
)

func TestTransformRestart_NoopNoPending(t *testing.T) {
	iof := commontest.LoadRuntimeFromFile(t, "vshn-postgres/restart/01-NoPendingReboot.yaml")
	require.NoError(t, runtime.AddToScheme(sgv1.SchemeBuilder.SchemeBuilder))

	res := TransformRestart(context.TODO(), iof)
	assert.Equal(t, fnv1aplha1.SeverityNormal, res.Resolve().Severity, res.Resolve().Message)

	comp := &vshnv1.XVSHNPostgreSQL{}
	err := iof.Desired.GetComposite(context.TODO(), comp)
	assert.NoError(t, err)

	assert.Len(t, iof.Desired.List(context.TODO()), 0)
}
func TestTransformRestart_NoopPendingOnRestart(t *testing.T) {
	iof := commontest.LoadRuntimeFromFile(t, "vshn-postgres/restart/02-PendingRebootNoRestart.yaml")
	require.NoError(t, runtime.AddToScheme(sgv1.SchemeBuilder.SchemeBuilder))

	res := TransformRestart(context.TODO(), iof)
	assert.Equal(t, fnv1aplha1.SeverityNormal, res.Resolve().Severity, res.Resolve().Message)

	comp := &vshnv1.XVSHNPostgreSQL{}
	err := iof.Desired.GetComposite(context.TODO(), comp)
	assert.NoError(t, err)

	assert.Len(t, iof.Desired.List(context.TODO()), 0)
}

func TestTransformRestart_RestartPending(t *testing.T) {
	iof := commontest.LoadRuntimeFromFile(t, "vshn-postgres/restart/02-PendingReboot.yaml")
	require.NoError(t, runtime.AddToScheme(sgv1.SchemeBuilder.SchemeBuilder))

	res := TransformRestart(context.TODO(), iof)
	assert.Equal(t, fnv1aplha1.SeverityNormal, res.Resolve().Severity, res.Resolve().Message)

	comp := &vshnv1.XVSHNPostgreSQL{}
	err := iof.Desired.GetComposite(context.TODO(), comp)
	assert.NoError(t, err)

	assert.Len(t, iof.Desired.List(context.TODO()), 1)

	sgrest := sgv1.SGDbOps{}
	assert.NoError(t, iof.Desired.GetFromObject(context.TODO(), &sgrest, "pgsql-gc9x4-pg-restart-1682587342"))
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
	iof := commontest.LoadRuntimeFromFile(t, "vshn-postgres/restart/03-KeepRecentReboots.yaml")
	require.NoError(t, runtime.AddToScheme(sgv1.SchemeBuilder.SchemeBuilder))

	now := func() time.Time {
		n, err := time.Parse(time.RFC3339, "2023-04-27T10:04:05Z")
		assert.NoError(t, err)
		return n
	}

	res := transformRestart(context.TODO(), iof, now)
	assert.Equal(t, fnv1aplha1.SeverityNormal, res.Resolve().Severity, res.Resolve().Message)

	comp := &vshnv1.XVSHNPostgreSQL{}
	err := iof.Desired.GetComposite(context.TODO(), comp)
	assert.NoError(t, err)

	assert.Len(t, iof.Desired.List(context.TODO()), 2)
}
