package vshnopenbao

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

func TestUpdateStatus_InitializedWhenSecretObserved(t *testing.T) {
	svc, comp := getOpenBaoTestCompWithInitSecret(t)
	ctx := context.TODO()

	assert.Nil(t, UpdateStatus(ctx, comp, svc))

	result := &vshnv1.XVSHNOpenBao{}
	assert.NoError(t, svc.GetDesiredComposite(result))
	assert.True(t, result.Status.Initialized)
}

func TestUpdateStatus_NotInitializedWhenSecretAbsent(t *testing.T) {
	svc, comp := getOpenBaoTestComp(t)
	ctx := context.TODO()

	assert.Nil(t, UpdateStatus(ctx, comp, svc))

	result := &vshnv1.XVSHNOpenBao{}
	assert.NoError(t, svc.GetDesiredComposite(result))
	assert.False(t, result.Status.Initialized)
}

func TestUpdateStatus_NotInitializedWhenInitDisabled(t *testing.T) {
	svc, comp := getOpenBaoTestCompWithInitDisabled(t)
	ctx := context.TODO()

	assert.Nil(t, UpdateStatus(ctx, comp, svc))

	result := &vshnv1.XVSHNOpenBao{}
	assert.NoError(t, svc.GetDesiredComposite(result))
	assert.False(t, result.Status.Initialized)
}
