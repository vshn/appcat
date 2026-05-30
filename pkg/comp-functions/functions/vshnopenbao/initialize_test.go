package vshnopenbao

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// TestOperatorInitializeFirstReconcile covers the case where no init secret exists yet.
// The function should create the RBAC and set up secret observers.
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

	// Init RBAC should be created.
	role := &rbacv1.Role{}
	assert.NoError(t, svc.GetDesiredKubeObject(role, "openbao-test-init-role"))
	rb := &rbacv1.RoleBinding{}
	assert.NoError(t, svc.GetDesiredKubeObject(rb, "openbao-test-init-role-binding"))

	// InitializationComplete must not be set in desired composite yet.
	desired := &vshnv1.VSHNOpenBao{}
	require.NoError(t, svc.GetDesiredComposite(desired))
	assert.False(t, desired.Status.InitializationComplete)
}

// TestOperatorInitializeSidecarWroteSecret covers the reconcile immediately after the init
// sidecar writes the root token secret. The function should set InitializationComplete=true.
func TestOperatorInitializeSidecarWroteSecret(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnopenbao/initialize/02_job_completed.yaml")
	comp := &vshnv1.VSHNOpenBao{}
	require.NoError(t, svc.GetObservedComposite(comp))

	result := InitializeCluster(context.Background(), comp, svc)
	assert.Nil(t, result)

	// Secret observers should still be set up.
	rootTokenObserver := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(rootTokenObserver, "openbao-test-init-output-observer"))

	// Init RBAC should still be present.
	role := &rbacv1.Role{}
	assert.NoError(t, svc.GetDesiredKubeObject(role, "openbao-test-init-role"))
	rb := &rbacv1.RoleBinding{}
	assert.NoError(t, svc.GetDesiredKubeObject(rb, "openbao-test-init-role-binding"))

	// InitializationComplete must be persisted to the desired composite.
	desired := &vshnv1.VSHNOpenBao{}
	require.NoError(t, svc.GetDesiredComposite(desired))
	assert.True(t, desired.Status.InitializationComplete)
}

// TestOperatorInitializeAlreadyInitialized covers the steady-state reconcile after the
// status flag has been persisted. RBAC should still be maintained.
func TestOperatorInitializeAlreadyInitialized(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnopenbao/initialize/03_already_initialized.yaml")
	comp := &vshnv1.VSHNOpenBao{}
	require.NoError(t, svc.GetObservedComposite(comp))

	result := InitializeCluster(context.Background(), comp, svc)
	assert.Nil(t, result)

	// Secret observers should still be set up so connection details keep flowing.
	rootTokenObserver := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(rootTokenObserver, "openbao-test-init-output-observer"))

	// Init RBAC must still be maintained (idempotent every reconcile).
	role := &rbacv1.Role{}
	assert.NoError(t, svc.GetDesiredKubeObject(role, "openbao-test-init-role"))
	rb := &rbacv1.RoleBinding{}
	assert.NoError(t, svc.GetDesiredKubeObject(rb, "openbao-test-init-role-binding"))

	// Status already true in observed — function does not call SetDesiredCompositeStatus again,
	// so the desired composite output has the zero value (false). The cluster state is preserved
	// by the Crossplane runtime, not by re-setting it each reconcile.
	desired := &vshnv1.VSHNOpenBao{}
	require.NoError(t, svc.GetDesiredComposite(desired))
	assert.False(t, desired.Status.InitializationComplete)
}

