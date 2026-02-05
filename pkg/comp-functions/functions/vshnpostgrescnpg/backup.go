package vshnpostgrescnpg

import (
	"context"
	"fmt"
	"maps"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/backup"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// Bootstrap backup (if enabled)
func SetupBackup(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, values map[string]any) error {
	// CreateObjectBucket has its own IsBackupEnabled to deal with bucket retention
	if err := backup.CreateObjectBucket(ctx, comp, svc); err != nil {
		return err
	}

	maintTime := common.SetRandomMaintenanceSchedule(comp)
	common.SetRandomBackupSchedule(comp, &maintTime)

	if err := svc.SetDesiredCompositeStatus(comp); err != nil {
		return fmt.Errorf("failed to set composite status: %w", err)
	}

	if comp.IsBackupEnabled() && comp.GetInstances() != 0 {
		// Deploy rclone encryption proxy and get backend credentials
		proxyCreds, err := backup.DeployRcloneProxy(ctx, svc, comp)
		if err != nil {
			return fmt.Errorf("cannot deploy rclone encryption proxy: %w", err)
		}

		// Configure barman cloud plugin via helm values with rclone proxy
		if proxyCreds != nil {
			if err := insertBackupValues(svc, comp, values, proxyCreds); err != nil {
				return err
			}
		}
	}
	return nil
}

// Add backup config to helm values
func insertBackupValues(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, values map[string]any, proxyCreds *backup.RcloneProxyCredentials) error {
	retention := comp.GetBackupRetention()
	retentionDays := retention.KeepDaily
	if retentionDays <= 0 {
		retentionDays = 6
	}

	// Enable the barman-cloud plugin in the cluster configuration
	clusterPlugins := []map[string]any{{
		"name":          "barman-cloud.cloudnative-pg.io",
		"enabled":       true,
		"isWALArchiver": true,
		"parameters": map[string]any{
			"barmanObjectName": "postgresql-object-store",
			"serverName":       "postgresql",
		},
	}}

	// Get existing cluster config or create it
	cluster, ok := values["cluster"].(map[string]any)
	if !ok {
		cluster = map[string]any{}
		values["cluster"] = cluster
	}
	cluster["plugins"] = clusterPlugins

	// Configure backups using the barman cloud plugin with rclone encryption proxy
	maps.Copy(values, map[string]any{
		"backups": map[string]any{
			"enabled":         true,
			"provider":        "s3",
			"endpointURL":     "http://rcloneproxy:9095",
			"region":          proxyCreds.Region,
			"retentionPolicy": fmt.Sprintf("%dd", retentionDays),
			"s3": map[string]any{
				// rclone gets confused when the bucket name matches the one it's rooted at. Since this can be arbitrary we hardcode it
				"bucket":    "backup",
				"region":    proxyCreds.Region,
				"path":      "/",
				"accessKey": proxyCreds.AccessID,
				"secretKey": proxyCreds.AccessKey,
			},
			"wal": map[string]any{
				"compression": "gzip",
				"maxParallel": 1,
			},
			"data": map[string]any{
				"compression": "gzip",
				"jobs":        2,
			},
			"secret": map[string]any{
				"create": true,
				"name":   "",
			},
			"scheduledBackups": []map[string]any{{
				"name":                 "default",
				"schedule":             transformSchedule(comp.GetBackupSchedule()),
				"backupOwnerReference": "cluster",
				"method":               "plugin",
				"pluginConfiguration": map[string]string{
					"name": "barman-cloud.cloudnative-pg.io",
				},
			}},
		},
	})

	return nil
}

// Transform backup schedule according to robfig/cron (used by CNPG)
// https://pkg.go.dev/github.com/robfig/cron#hdr-CRON_Expression_Format
func transformSchedule(thisSchedule string) string {
	return fmt.Sprintf("0 %s", thisSchedule)
}
