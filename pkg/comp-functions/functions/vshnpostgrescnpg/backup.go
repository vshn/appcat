package vshnpostgrescnpg

import (
	"context"
	"fmt"
	"maps"
	"strings"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/backup"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// Backup bucket connection details
type backupCredentials struct {
	endpoint  string
	bucket    string
	region    string
	accessId  string
	accessKey string
}

// Bootstrap backup (if enabled)
func SetupBackup(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, values map[string]any) error {
	// CreateObjectBucket has its own IsBackupEnabled to deal with bucket retention
	if err := backup.CreateObjectBucket(ctx, comp, svc); err != nil {
		return err
	}

	if comp.IsBackupEnabled() {
		setSchedules(comp)
		if err := insertBackupValues(svc, comp, values); err != nil {
			return err
		}
	}
	return nil
}

// Add backup config to helm values
func insertBackupValues(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, values map[string]any) error {
	connectionDetails, err := getBackupBucketConnectionDetails(svc, comp)
	if err != nil {
		return err
	}

	retention := comp.GetBackupRetention()
	retentionDays := retention.KeepDaily
	if retentionDays <= 0 {
		retentionDays = 6
	}

	maps.Copy(values, map[string]any{
		"backups": map[string]any{
			"enabled":         true,
			"endpointURL":     connectionDetails.endpoint,
			"retentionPolicy": fmt.Sprintf("%dd", retentionDays),
			"scheduledBackups": []map[string]string{{
				"name":                 "default",
				"method":               "barmanObjectStore",
				"schedule":             transformSchedule(comp.GetBackupSchedule()),
				"backupOwnerReference": "self",
			}},
			"data": map[string]string{
				"encryption": "",
			},
			"wal": map[string]string{
				"encryption": "",
			},
			"s3": map[string]string{
				"bucket":    connectionDetails.bucket,
				"region":    connectionDetails.region,
				"accessKey": connectionDetails.accessId,
				"secretKey": connectionDetails.accessKey,
				// The S3 secret MUST have the keys ACCESS_KEY_ID and ACCESS_SECRET_KEY.
				// This is currently hardcoded in the chart, and as the CD secret does not have those keys in verbatim,
				// we are forced to pass those values to the chart and letting it create its own secret instead.
			},
		},
	})

	return nil
}

func getBackupBucketConnectionDetails(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL) (backupCredentials, error) {
	backupCredentials := backupCredentials{}
	cd, err := svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + "-backup")
	if err != nil && err == runtime.ErrNotFound {
		return backupCredentials, fmt.Errorf("backup bucket connection details not found")
	} else if err != nil {
		return backupCredentials, err
	}

	endpoint, _ := strings.CutSuffix(string(cd["ENDPOINT_URL"]), "/")
	backupCredentials.endpoint = endpoint
	backupCredentials.bucket = string(cd["BUCKET_NAME"])
	backupCredentials.region = string(cd["AWS_REGION"])
	backupCredentials.accessId = string(cd["AWS_ACCESS_KEY_ID"])
	backupCredentials.accessKey = string(cd["AWS_SECRET_ACCESS_KEY"])
	return backupCredentials, nil
}

func setSchedules(comp *vshnv1.VSHNPostgreSQL) {
	maintTime := common.SetRandomMaintenanceSchedule(comp)
	common.SetRandomBackupSchedule(comp, &maintTime)
}

// Transform backup schedule according to robfig/cron (used by CNPG)
// https://pkg.go.dev/github.com/robfig/cron#hdr-CRON_Expression_Format
func transformSchedule(thisSchedule string) string {
	return fmt.Sprintf("0 %s", thisSchedule)
}
