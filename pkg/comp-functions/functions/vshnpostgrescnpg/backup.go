package vshnpostgrescnpg

import (
	"context"
	"fmt"
	"strings"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/backup"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// CNPG scheduled backup spec
type scheduledBackup struct {
	Name                 string `json:"name"`
	Schedule             string `json:"schedule"`
	BackupOwnerReference string `json:"backupOwnerReference"`
	Method               string `json:"method"`
}

// Backup bucket connection details
type backupCredentials struct {
	endpoint  string
	bucket    string
	region    string
	accessId  string
	accessKey string
}

// Bootstrap backup
func SetupBackup(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, values map[string]any) error {
	// CreateObjectBucket has its own IsBackupEnabled to deal with bucket retention
	if err := backup.CreateObjectBucket(ctx, comp, svc); err != nil {
		return err
	}

	if comp.IsBackupEnabled() {
		if err := insertBackupValues(svc, comp, values); err != nil {
			return err
		}
	}
	return nil
}

// Add backup config to helm values
func insertBackupValues(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, values map[string]any) error {
	setSchedules(comp)
	svc.Log.Info("Schedules",
		"scheduleBackup", comp.GetBackupSchedule(),
		"maintenanceSchedule", comp.GetFullMaintenanceSchedule(),
	)

	connectionDetails, err := getBackupBucketConnectionDetails(svc, comp)
	if err != nil {
		return err
	}

	scheduledBackups := []scheduledBackup{{
		Name:                 "default",
		Schedule:             comp.GetBackupSchedule(),
		BackupOwnerReference: "self",
		Method:               "barmanObjectStore",
	}}

	retention := comp.GetBackupRetention()
	retentionDays := retention.KeepDaily
	if retentionDays <= 0 {
		retentionDays = 6
	}

	for key, value := range map[string]any{
		"backups.enabled":          true,
		"backups.endpointURL":      connectionDetails.endpoint,
		"backups.retentionPolicy":  fmt.Sprintf("%dd", retentionDays),
		"backups.scheduledBackups": scheduledBackups,
		"backups.s3.bucket":        connectionDetails.bucket,
		"backups.s3.region":        connectionDetails.region,
		"backups.s3.accessKey":     connectionDetails.accessId,
		"backups.s3.secretKey":     connectionDetails.accessKey,
		// The S3 secret MUST have the keys ACCESS_KEY_ID and ACCESS_SECRET_KEY.
		// This is currently hardcoded in the chart, and as the CD secret does not have those keys in verbatim,
		// we are forced to pass those values to the chart and letting it create its own secret instead.
	} {
		err := common.SetNestedObjectValue(values, strings.Split(key, "."), value)
		if err != nil {
			return fmt.Errorf("cannot set '%v' to '%v': %w", key, value, err)
		}
	}

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
