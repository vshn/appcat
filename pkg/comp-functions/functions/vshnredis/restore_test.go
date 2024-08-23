package vshnredis

// test cases for the function RestoreBackup

import (
	"context"

	"testing"

	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
)

func TestRestoreBackup_NoConfig(t *testing.T) {
	ctx := context.Background()
	expectResult := runtime.NewWarningResult("Composite is missing backupName or claimName namespace, skipping transformation")

	t.Run("WhenNoRestore_ThenNoErrorAndNoChanges", func(t *testing.T) {

		// Given
		io := commontest.LoadRuntimeFromFile(t, "vshnredis/restore/01-GivenNoRestoreConfig.yaml")

		// When
		result := RestoreBackup(ctx, &vshnv1.VSHNRedis{}, io)

		// Then
		assert.Equal(t, expectResult, result)
	})
}

func TestRestoreBackup_IncompleteConfig(t *testing.T) {
	ctx := context.Background()

	expectResultCN := runtime.NewWarningResult("Composite is missing backupName or claimName namespace, skipping transformation")

	t.Run("WhenNoRestore_ThenNoErrorAndNoChanges", func(t *testing.T) {

		// Given
		io := commontest.LoadRuntimeFromFile(t, "vshnredis/restore/01-GivenRestoreConfigNoCN.yaml")
		// When
		resultCN := RestoreBackup(ctx, &vshnv1.VSHNRedis{}, io)

		// Then
		assert.Equal(t, expectResultCN, resultCN)

		// Given
		io = commontest.LoadRuntimeFromFile(t, "vshnredis/restore/01-GivenRestoreConfigNoBN.yaml")

		// When
		resultBN := RestoreBackup(ctx, &vshnv1.VSHNRedis{}, io)

		// Then
		assert.Equal(t, expectResultCN, resultBN)
	})
}

func TestRestoreBackup(t *testing.T) {
	ctx := context.Background()

	// return Normal and new job resources in Desired
	io := commontest.LoadRuntimeFromFile(t, "vshnredis/restore/01-GivenRestoreConfig.yaml")

	result := RestoreBackup(ctx, &vshnv1.VSHNRedis{}, io)
	assert.Nil(t, result)

	resNamePrepJob := "redis-gc9x4-bar-prepare-job"
	kubeObjectPrepJob := &xkube.Object{}
	assert.NoError(t, io.GetDesiredComposedResourceByName(kubeObjectPrepJob, resNamePrepJob))

	j := &batchv1.Job{}

	assert.NoError(t, yaml.Unmarshal(kubeObjectPrepJob.Spec.ForProvider.Manifest.Raw, j))
	assert.Equal(t, resNamePrepJob, j.ObjectMeta.Name)
}
