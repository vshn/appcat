package vshnopenbao

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	xhelmbeta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestDeployOpenBao(t *testing.T) {
	svc, comp := getOpenBaoTestComp(t)

	ctx := context.TODO()

	assert.Nil(t, BootstrapNamespace(ctx, comp, svc))
	assert.Nil(t, DeployOpenBao(ctx, comp, svc))

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetObservedKubeObject(ns, "openbao-test-ns"))

	r := &xhelmbeta1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(r, "openbao-test-release"))

	var values map[string]interface{}
	assert.NoError(t, json.Unmarshal(r.Spec.ForProvider.Values.Raw, &values))

	_, hasExtraContainers := values["server"].(map[string]interface{})["extraContainers"]
	assert.False(t, hasExtraContainers, "extraContainers should not be present in Helm values")
}

func TestInitOpenBao_CreatesJobWhenNotInitialized(t *testing.T) {
	svc, comp := getOpenBaoTestComp(t)
	ctx := context.TODO()

	assert.Nil(t, DeployOpenBao(ctx, comp, svc))
	assert.Nil(t, InitOpenBao(ctx, comp, svc))

	job := &batchv1.Job{}
	assert.NoError(t, svc.GetDesiredKubeObject(job, "openbao-test-init"))
	assert.Equal(t, "openbao-test-init", job.Name)
	assert.Equal(t, "vshn-openbao-openbao-test", job.Namespace)
	assert.Equal(t, "openbao-test", job.Spec.Template.Spec.ServiceAccountName)
	assert.EqualValues(t, 3, *job.Spec.BackoffLimit)
	assert.EqualValues(t, 86400, *job.Spec.TTLSecondsAfterFinished)
}

func TestInitOpenBao_SkipsJobWhenAlreadyInitialized(t *testing.T) {
	ctx := context.TODO()

	svc, comp := getOpenBaoTestCompWithInitSecret(t)
	assert.Nil(t, DeployOpenBao(ctx, comp, svc))
	assert.Nil(t, InitOpenBao(ctx, comp, svc))

	job := &batchv1.Job{}
	assert.ErrorIs(t, svc.GetDesiredKubeObject(job, "openbao-test-init"), runtime.ErrNotFound,
		"init job should not be in desired state when already initialized")
}

func TestInitOpenBao_SkipsJobWhenStatusInitialized(t *testing.T) {
	ctx := context.TODO()

	svc, comp := getOpenBaoTestCompWithStatusInitialized(t)
	assert.Nil(t, DeployOpenBao(ctx, comp, svc))
	assert.Nil(t, InitOpenBao(ctx, comp, svc))

	job := &batchv1.Job{}
	assert.ErrorIs(t, svc.GetDesiredKubeObject(job, "openbao-test-init"), runtime.ErrNotFound,
		"init job should not be in desired state when status.initialized is true")
}
