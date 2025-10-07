package vshnpostgrescnpg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func Test_BackupBootstrapDisabled(t *testing.T) {
	svc, comp := getPostgreSqlComp(t, "vshn-postgres/deploy/05_backup_disabled_cnpg.yaml")
	ctx := context.TODO()

	// If backup has been disabled and the instance first created, expect neither bucket nor backup values
	values, err := createCnpgHelmValues(ctx, svc, comp)
	assert.NoError(t, err)

	assert.NoError(t, SetupBackup(ctx, svc, comp, values))
	assert.Nil(t, values["backups"])

	bucketName := comp.GetName() + "-backup"
	err = svc.GetDesiredComposedResourceByName(&appcatv1.XObjectBucket{}, bucketName)
	assert.ErrorIs(t, err, runtime.ErrNotFound)
}

func TestBackupBooststrapEnabled(t *testing.T) {
	svc, comp := getPostgreSqlComp(t, "vshn-postgres/deploy/05_backup_cnpg.yaml")
	ctx := context.TODO()

	// If backup has been enabled, expect values and backup bucket
	values, err := createCnpgHelmValues(ctx, svc, comp)
	assert.NoError(t, err)

	assert.NoError(t, SetupBackup(ctx, svc, comp, values))
	assert.NotNil(t, values["backups"])

	backupValues := values["backups"].(map[string]any)

	// Enabled, retention
	assert.True(t, backupValues["enabled"].(bool))
	assert.Equal(t, backupValues["retentionPolicy"], "6d")

	// Schedule
	assert.Equal(t,
		backupValues["scheduledBackups"].([]map[string]string)[0]["schedule"],
		transformSchedule(comp.GetBackupSchedule()),
	)

	// Bucket configuration
	cd, err := getBackupBucketConnectionDetails(svc, comp)
	assert.NoError(t, err)
	assert.Equal(t, cd.endpoint, "https://s3.minio.local") // No trailing /
	assert.Equal(t, cd.bucket, "backupBucket")
	assert.Equal(t, cd.region, "rma")
	assert.Equal(t, cd.accessId, "secretAccessId")
	assert.Equal(t, cd.accessKey, "secretAccessKey")

	assert.Equal(t, cd.endpoint, backupValues["endpointURL"])
	assert.Equal(t, cd.bucket, backupValues["s3"].(map[string]string)["bucket"])
	assert.Equal(t, cd.region, backupValues["s3"].(map[string]string)["region"])
	assert.Equal(t, cd.accessId, backupValues["s3"].(map[string]string)["accessKey"])
	assert.Equal(t, cd.accessKey, backupValues["s3"].(map[string]string)["secretKey"])

	bucketName := comp.GetName() + "-backup"
	err = svc.GetDesiredComposedResourceByName(&appcatv1.XObjectBucket{}, bucketName)
	assert.NoError(t, err)
}
