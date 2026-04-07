package vshnpostgrescnpg

import (
	"context"
	"fmt"
	"maps"
	"strings"

	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/backup"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
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

	maintTime := common.SetRandomMaintenanceSchedule(comp)
	common.SetRandomBackupSchedule(comp, &maintTime)

	if err := svc.SetDesiredCompositeStatus(comp); err != nil {
		return fmt.Errorf("failed to set composite status: %w", err)
	}

	if comp.IsBackupEnabled() && comp.GetInstances() != 0 {
		// Configure barman cloud plugin via helm values
		if err := insertBackupValues(ctx, svc, comp, values); err != nil {
			return err
		}
	}
	return nil
}

// Add backup config to helm values
func insertBackupValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, values map[string]any) error {
	connectionDetails, err := getBackupBucketConnectionDetails(ctx, svc, comp)
	if err != nil {
		return err
	}

	retention := comp.GetBackupRetention()
	retentionDays := retention.KeepDaily
	if retentionDays <= 0 {
		retentionDays = 6
	}

	// Enable the barman-cloud plugin in the cluster configuration
	// serverName determines the path within the object store: {destinationPath}/{serverName}/
	// By including the major version, each major version upgrade writes to a separate path,
	// preventing WAL timeline conflicts after pg_upgrade resets the timeline to 1.
	clusterPlugins := []map[string]any{{
		"name":          "barman-cloud.cloudnative-pg.io",
		"enabled":       true,
		"isWALArchiver": true,
		"parameters": map[string]any{
			"barmanObjectName": "postgresql-object-store",
			"serverName":       "postgresql-" + comp.Spec.Parameters.Service.MajorVersion,
		},
	}}

	// Get existing cluster config or create it
	cluster, ok := values["cluster"].(map[string]any)
	if !ok {
		cluster = map[string]any{}
		values["cluster"] = cluster
	}
	cluster["plugins"] = clusterPlugins

	// Configure backups using the barman cloud plugin
	maps.Copy(values, map[string]any{
		"backups": map[string]any{
			"enabled":         true,
			"provider":        "s3",
			"endpointURL":     connectionDetails.endpoint,
			"region":          connectionDetails.region,
			"retentionPolicy": fmt.Sprintf("%dd", retentionDays),
			"s3": map[string]any{
				"bucket":    connectionDetails.bucket,
				"region":    connectionDetails.region,
				"path":      "/",
				"accessKey": connectionDetails.accessId,
				"secretKey": connectionDetails.accessKey,
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

func getBackupBucketConnectionDetails(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL) (backupCredentials, error) {
	backupCredentials := backupCredentials{}

	cd := map[string][]byte{}

	if ub := comp.GetUnmanagedBucket(); ub != nil {
		secret := &corev1.Secret{}

		kubeCDs := []xkube.ConnectionDetail{
			{
				ToConnectionSecretKey: "AWS_ACCESS_KEY_ID",
				ObjectReference: corev1.ObjectReference{
					Kind:       "Secret",
					APIVersion: "v1",
					Namespace:  comp.GetClaimNamespace(),
					Name:       ub.AccessKey.Name,
					FieldPath:  fmt.Sprintf("data.%s", ub.AccessKey.Key),
				},
			},
			{
				ToConnectionSecretKey: "AWS_SECRET_ACCESS_KEY",
				ObjectReference: corev1.ObjectReference{
					Kind:       "Secret",
					APIVersion: "v1",
					Namespace:  comp.GetClaimNamespace(),
					Name:       ub.SecretKey.Name,
					FieldPath:  fmt.Sprintf("data.%s", ub.SecretKey.Key),
				},
			},
		}

		_, err := svc.CopyKubeResource(ctx, secret, comp.GetName()+"-bucket-credentials", ub.AccessKey.Name, comp.GetClaimNamespace(), comp.GetInstanceNamespace(), kubeCDs...)
		if err != nil {
			return backupCredentials, err
		}

		cd, err = svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + "-bucket-credentials-claim-observer")
		if err != nil && err == runtime.ErrNotFound {
			return backupCredentials, fmt.Errorf("backup bucket connection details not found")
		} else if err != nil {
			return backupCredentials, err
		}

		if cd == nil {
			// On first reconcile the connection details will be nil
			cd = map[string][]byte{}
		}

		cd["ENDPOINT_URL"] = []byte(ub.Endpoint)
		cd["BUCKET_NAME"] = []byte(ub.Bucket)
		cd["AWS_REGION"] = []byte(ub.Region)

	} else {
		var err error
		cd, err = svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + "-backup")
		if err != nil && err == runtime.ErrNotFound {
			return backupCredentials, fmt.Errorf("backup bucket connection details not found")
		} else if err != nil {
			return backupCredentials, err
		}

	}

	endpoint, _ := strings.CutSuffix(string(cd["ENDPOINT_URL"]), "/")
	backupCredentials.endpoint = endpoint
	backupCredentials.bucket = string(cd["BUCKET_NAME"])
	backupCredentials.region = string(cd["AWS_REGION"])
	backupCredentials.accessId = string(cd["AWS_ACCESS_KEY_ID"])
	backupCredentials.accessKey = string(cd["AWS_SECRET_ACCESS_KEY"])
	return backupCredentials, nil
}

// Transform backup schedule according to robfig/cron (used by CNPG)
// https://pkg.go.dev/github.com/robfig/cron#hdr-CRON_Expression_Format
func transformSchedule(thisSchedule string) string {
	return fmt.Sprintf("0 %s", thisSchedule)
}
