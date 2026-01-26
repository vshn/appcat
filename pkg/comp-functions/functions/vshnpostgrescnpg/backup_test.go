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

	// Check backups section exists
	assert.NotNil(t, values["backups"])
	backupValues := values["backups"].(map[string]any)

	// Check backup configuration
	assert.True(t, backupValues["enabled"].(bool))
	assert.Equal(t, "s3", backupValues["provider"])
	assert.Equal(t, "6d", backupValues["retentionPolicy"])

	// Bucket configuration
	cd, err := getBackupBucketConnectionDetails(svc, comp)
	assert.NoError(t, err)
	assert.Equal(t, cd.endpoint, "https://s3.minio.local") // No trailing /
	assert.Equal(t, cd.bucket, "backupBucket")
	assert.Equal(t, cd.region, "rma")
	assert.Equal(t, cd.accessId, "secretAccessId")
	assert.Equal(t, cd.accessKey, "secretAccessKey")

	// Check endpoint, region (top-level), and S3 configuration
	assert.Equal(t, cd.endpoint, backupValues["endpointURL"])
	assert.Equal(t, cd.region, backupValues["region"])
	s3Config := backupValues["s3"].(map[string]any)
	assert.Equal(t, cd.bucket, s3Config["bucket"])
	assert.Equal(t, cd.region, s3Config["region"])
	assert.Equal(t, "/", s3Config["path"])
	assert.Equal(t, cd.accessId, s3Config["accessKey"])
	assert.Equal(t, cd.accessKey, s3Config["secretKey"])

	// Check WAL and data configuration
	walConfig := backupValues["wal"].(map[string]any)
	assert.Equal(t, "gzip", walConfig["compression"])
	assert.Equal(t, 1, walConfig["maxParallel"])

	dataConfig := backupValues["data"].(map[string]any)
	assert.Equal(t, "gzip", dataConfig["compression"])
	assert.Equal(t, 2, dataConfig["jobs"])

	// Check secret configuration
	secretConfig := backupValues["secret"].(map[string]any)
	assert.True(t, secretConfig["create"].(bool))
	assert.Equal(t, "", secretConfig["name"])

	// Check cluster plugins configuration
	assert.NotNil(t, values["cluster"])
	clusterConfig := values["cluster"].(map[string]any)
	assert.NotNil(t, clusterConfig["plugins"])
	plugins := clusterConfig["plugins"].([]map[string]any)
	assert.Len(t, plugins, 1)
	assert.Equal(t, "barman-cloud.cloudnative-pg.io", plugins[0]["name"])
	assert.True(t, plugins[0]["enabled"].(bool))
	assert.True(t, plugins[0]["isWALArchiver"].(bool))
	pluginParams := plugins[0]["parameters"].(map[string]any)
	assert.Equal(t, "postgresql-object-store", pluginParams["barmanObjectName"])
	assert.Equal(t, "", pluginParams["serverName"])

	// Check scheduled backups
	scheduledBackups := backupValues["scheduledBackups"].([]map[string]any)
	assert.Len(t, scheduledBackups, 1)
	assert.Equal(t, "default", scheduledBackups[0]["name"])
	assert.Equal(t, transformSchedule(comp.GetBackupSchedule()), scheduledBackups[0]["schedule"])
	assert.Equal(t, "cluster", scheduledBackups[0]["backupOwnerReference"])
	assert.Equal(t, "plugin", scheduledBackups[0]["method"])

	// Check plugin configuration
	pluginConfig := scheduledBackups[0]["pluginConfiguration"].(map[string]string)
	assert.Equal(t, "barman-cloud.cloudnative-pg.io", pluginConfig["name"])

	bucketName := comp.GetName() + "-backup"
	err = svc.GetDesiredComposedResourceByName(&appcatv1.XObjectBucket{}, bucketName)
	assert.NoError(t, err)
}
