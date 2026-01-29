package vshnpostgrescnpg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
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

	// Check that rclone proxy endpoint is used (not direct S3)
	assert.Equal(t, "http://rcloneproxy:9095", backupValues["endpointURL"])

	// Region and credentials are passed through from the bucket
	assert.Equal(t, "rma", backupValues["region"])
	s3Config := backupValues["s3"].(map[string]any)
	// Bucket is hardcoded because rclone gets confused when the bucket name matches the backend bucket
	assert.Equal(t, "backup", s3Config["bucket"])
	assert.Equal(t, "rma", s3Config["region"])
	assert.Equal(t, "/", s3Config["path"])
	assert.Equal(t, "secretAccessId", s3Config["accessKey"])
	assert.Equal(t, "secretAccessKey", s3Config["secretKey"])

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

	// Check that backup bucket is created
	bucketName := comp.GetName() + "-backup"
	err = svc.GetDesiredComposedResourceByName(&appcatv1.XObjectBucket{}, bucketName)
	assert.NoError(t, err)

	// Check that rclone proxy Helm release is created
	rcloneReleaseName := comp.GetName() + "-rclone"
	rcloneRelease := &xhelmv1.Release{}
	err = svc.GetDesiredComposedResourceByName(rcloneRelease, rcloneReleaseName)
	assert.NoError(t, err, "rclone proxy Helm release should be created")
	assert.Equal(t, comp.GetInstanceNamespace(), rcloneRelease.Spec.ForProvider.Namespace)
}
