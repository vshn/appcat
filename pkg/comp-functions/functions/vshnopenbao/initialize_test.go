package vshnopenbao

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// TestOperatorInitializeFirstReconcile covers the case where no init secret exists yet.
// The function should create the RBAC and init job.
func TestOperatorInitializeFirstReconcile(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnopenbao/initialize/01_first_reconcile.yaml")
	comp := &vshnv1.VSHNOpenBao{}
	require.NoError(t, svc.GetObservedComposite(comp))

	result := InitializeCluster(context.Background(), comp, svc)
	assert.Nil(t, result)

	// Secret observers should always be set up.
	rootTokenObserver := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(rootTokenObserver, "openbao-test-init-output-observer"))
	unsealKeysObserver := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(unsealKeysObserver, "openbao-test-unseal-keys-observer"))

	// SA and init job should be created.
	sa := &corev1.ServiceAccount{}
	assert.NoError(t, svc.GetDesiredKubeObject(sa, "openbao-test-openbao-init-serviceaccount"))
	job := &batchv1.Job{}
	assert.NoError(t, svc.GetDesiredKubeObject(job, "openbao-test-init-job"))
	assert.Equal(t, "vshn-openbao-openbao-test", job.Namespace)
	assert.Equal(t, "bitnami/kubectl:latest", job.Spec.Template.Spec.Containers[0].Image)

	// InitializationComplete must not be set in desired composite yet.
	desired := &vshnv1.VSHNOpenBao{}
	require.NoError(t, svc.GetDesiredComposite(desired))
	assert.False(t, desired.Status.InitializationComplete)
}

// TestOperatorInitializeJobCompleted covers the reconcile immediately after the init job
// writes the root token secret. The function should set InitializationComplete=true and
// skip creating the job.
func TestOperatorInitializeJobCompleted(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnopenbao/initialize/02_job_completed.yaml")
	comp := &vshnv1.VSHNOpenBao{}
	require.NoError(t, svc.GetObservedComposite(comp))

	result := InitializeCluster(context.Background(), comp, svc)
	assert.Nil(t, result)

	// Secret observers should still be set up.
	rootTokenObserver := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(rootTokenObserver, "openbao-test-init-output-observer"))

	// Job and SA must not be in the desired state.
	job := &batchv1.Job{}
	assert.ErrorIs(t, svc.GetDesiredKubeObject(job, "openbao-test-init-job"), runtime.ErrNotFound)
	sa := &corev1.ServiceAccount{}
	assert.ErrorIs(t, svc.GetDesiredKubeObject(sa, "sa-openbao-init"), runtime.ErrNotFound)

	// InitializationComplete must be persisted to the desired composite.
	desired := &vshnv1.VSHNOpenBao{}
	require.NoError(t, svc.GetDesiredComposite(desired))
	assert.True(t, desired.Status.InitializationComplete)
}

// TestOperatorInitializeAlreadyInitialized covers the steady-state reconcile after the
// status flag has been persisted. The function should be a no-op for the job and RBAC.
func TestOperatorInitializeAlreadyInitialized(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnopenbao/initialize/03_already_initialized.yaml")
	comp := &vshnv1.VSHNOpenBao{}
	require.NoError(t, svc.GetObservedComposite(comp))

	result := InitializeCluster(context.Background(), comp, svc)
	assert.Nil(t, result)

	// Secret observers should still be set up so connection details keep flowing.
	rootTokenObserver := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(rootTokenObserver, "openbao-test-init-output-observer"))

	// Job and SA must not be recreated.
	job := &batchv1.Job{}
	assert.ErrorIs(t, svc.GetDesiredKubeObject(job, "openbao-test-init-job"), runtime.ErrNotFound)
	sa := &corev1.ServiceAccount{}
	assert.ErrorIs(t, svc.GetDesiredKubeObject(sa, "sa-openbao-init"), runtime.ErrNotFound)
}
