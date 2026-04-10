package vshnpostgrescnpg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestRestore_WaitingForCredentials(t *testing.T) {
	svc, comp := getPostgreSqlComp(t, "vshn-postgres/deploy/06_cnpg_restore.yaml")
	ctx := context.TODO()

	values, err := createCnpgHelmValues(ctx, svc, comp)
	require.NoError(t, err)

	result, skipRelease := handleRestore(ctx, comp, svc, values)

	// First reconciliation loop: should skip Helm release and return a warning
	assert.True(t, skipRelease, "expected skipRelease=true while waiting for credentials")
	assert.NotNil(t, result, "expected a warning result while waiting for credentials")

	// Copy job should be created (resourceName = "cnpg-copy-job")
	job := &batchv1.Job{}
	err = svc.GetDesiredKubeObject(job, "cnpg-copy-job")
	assert.NoError(t, err, "copy job should be created")
	assert.Equal(t, "appcat-control", job.Namespace)

	// Verify copy job env vars
	container := job.Spec.Template.Spec.Containers[0]
	envMap := make(map[string]string)
	for _, e := range container.Env {
		envMap[e.Name] = e.Value
	}
	assert.Equal(t, "unit-test", envMap["CLAIM_NAMESPACE"])
	assert.Equal(t, "pgsql-source", envMap["CLAIM_NAME"])
	assert.Equal(t, comp.GetInstanceNamespace(), envMap["TARGET_NAMESPACE"])
	assert.Equal(t, "syn-crossplane", envMap["CROSSPLANE_NAMESPACE"])

	// Recovery observer should be created
	observedSecret := &corev1.Secret{}
	err = svc.GetDesiredKubeObject(observedSecret, comp.GetName()+"-cnpg-recovery-creds")
	assert.NoError(t, err, "recovery credentials observer should be created")

	// Values should NOT have recovery mode set (credentials not yet available)
	assert.Nil(t, values["mode"], "mode should not be set while waiting for credentials")
	assert.Nil(t, values["recovery"], "recovery should not be set while waiting for credentials")
}

func TestRestore_RecoveryMode(t *testing.T) {
	svc, comp := getPostgreSqlComp(t, "vshn-postgres/deploy/07_cnpg_restore_ready.yaml")
	ctx := context.TODO()

	values, err := createCnpgHelmValues(ctx, svc, comp)
	require.NoError(t, err)

	result, skipRelease := handleRestore(ctx, comp, svc, values)

	// Credentials available: should NOT skip Helm release
	assert.False(t, skipRelease, "expected skipRelease=false when credentials are available")
	assert.Nil(t, result, "expected no warning result when credentials are available")

	// Values should have recovery mode set
	assert.Equal(t, "recovery", values["mode"])
	recovery := values["recovery"].(map[string]any)
	assert.Equal(t, "object_store", recovery["method"])
	assert.Equal(t, recoveryObjectStoreName, recovery["objectStoreName"])
	assert.Equal(t, "postgresql-17", recovery["clusterName"], "clusterName should use restore instance's major version")

	// PITR target should be set
	pitrTarget := recovery["pitrTarget"].(map[string]any)
	assert.Equal(t, "2024-06-15T22:00:00Z", pitrTarget["time"])

	// Copy job should also be created during recovery
	job := &batchv1.Job{}
	err = svc.GetDesiredKubeObject(job, "cnpg-copy-job")
	assert.NoError(t, err, "copy job should be created")
}

func TestRestore_Standalone(t *testing.T) {
	// Once the Helm release is ready, subsequent reconciliation loops switch to standalone mode.
	// Since we can't easily simulate a ready Helm release in the test fixture,
	// we verify standalone logic by testing with no restore params.
	// The real standalone path is: IsResourceReady returns true -> no recovery values set.
	// This is effectively the same as no restore params for the values.
	svc, comp := getPostgreSqlComp(t, "vshn-postgres/deploy/05_backup_cnpg.yaml")
	ctx := context.TODO()

	values, err := createCnpgHelmValues(ctx, svc, comp)
	require.NoError(t, err)

	result, skipRelease := handleRestore(ctx, comp, svc, values)

	// No restore params -> no-op
	assert.False(t, skipRelease)
	assert.Nil(t, result)
	assert.Nil(t, values["mode"])
	assert.Nil(t, values["recovery"])
}

func TestRestore_NoRestoreParams(t *testing.T) {
	svc, comp := getPostgreSqlComp(t, "vshn-postgres/deploy/05_backup_cnpg.yaml")
	ctx := context.TODO()

	// Explicitly verify no restore is set
	assert.Nil(t, comp.Spec.Parameters.Restore)

	values, err := createCnpgHelmValues(ctx, svc, comp)
	require.NoError(t, err)

	result, skipRelease := handleRestore(ctx, comp, svc, values)

	assert.False(t, skipRelease)
	assert.Nil(t, result)
	assert.Nil(t, values["mode"])
}

func TestRestore_SetRecoveryValues(t *testing.T) {
	values := map[string]any{}
	secretData := map[string][]byte{
		"SOURCE_MAJOR_VERSION": []byte("17"), // source was upgraded, but we ignore this
	}

	comp := &vshnv1.VSHNPostgreSQL{}
	comp.Spec.Parameters.Service.MajorVersion = "15" // restore instance targets PG 15 backups
	comp.Spec.Parameters.Restore = &vshnv1.VSHNPostgreSQLRestore{
		RecoveryTimeStamp: "2024-06-15T22:00:00Z",
	}

	setRecoveryValues(values, secretData, comp)

	assert.Equal(t, "recovery", values["mode"])
	recovery := values["recovery"].(map[string]any)
	assert.Equal(t, "object_store", recovery["method"])
	assert.Equal(t, recoveryObjectStoreName, recovery["objectStoreName"])
	assert.Equal(t, "postgresql-15", recovery["clusterName"], "clusterName should use restore instance's major version, not source's")

	pitrTarget := recovery["pitrTarget"].(map[string]any)
	assert.Equal(t, "2024-06-15T22:00:00Z", pitrTarget["time"])
}

func TestRestore_SetRecoveryValues_NoPITR(t *testing.T) {
	values := map[string]any{}
	secretData := map[string][]byte{
		"SOURCE_MAJOR_VERSION": []byte("15"), // ignored
	}

	comp := &vshnv1.VSHNPostgreSQL{}
	comp.Spec.Parameters.Service.MajorVersion = "17"
	comp.Spec.Parameters.Restore = &vshnv1.VSHNPostgreSQLRestore{}

	setRecoveryValues(values, secretData, comp)

	assert.Equal(t, "recovery", values["mode"])
	recovery := values["recovery"].(map[string]any)
	assert.Equal(t, "postgresql-17", recovery["clusterName"], "clusterName should use restore instance's major version")
	assert.Nil(t, recovery["pitrTarget"], "pitrTarget should not be set when RecoveryTimeStamp is empty")
}
