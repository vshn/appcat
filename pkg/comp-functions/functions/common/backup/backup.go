package backup

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/sethvargo/go-password/password"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
	credentialSecretName         = "backup-bucket-credentials"
	k8upRepoSecretName           = "k8up-repository-password"
	k8upRepoSecretKey            = "password"
	backupScriptCMName           = "backup-script"
	BackupDisabledTimestampLabel = "appcat.vshn.io/backup-disabled-timestamp"
)

// AddK8upBackup creates an S3 bucket and a K8up schedule according to the composition spec.
// When backup is disabled, it only creates/preserves the bucket for retention but skips other backup objects.
func AddK8upBackup(ctx context.Context, svc *runtime.ServiceRuntime, comp common.InfoGetter) error {

	l := controllerruntime.LoggerFrom(ctx)

	// Always create/preserve the backup bucket (handles both enabled and disabled states)
	l.Info("Creating/preserving backup bucket", "backupEnabled", comp.IsBackupEnabled())
	err := CreateObjectBucket(ctx, comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create/preserve backup bucket: %w", err)
	}

	// Check if bucket exists by looking for observed bucket or if backup is enabled
	bucketExists := comp.IsBackupEnabled()
	if !bucketExists {
		// Check if bucket exists in observed state (when backup was previously enabled)
		observedBucket := &appcatv1.XObjectBucket{}
		bucketName := comp.GetName() + "-backup"
		err := svc.GetObservedComposedResource(observedBucket, bucketName)
		if err != nil && err != runtime.ErrNotFound {
			return fmt.Errorf("cannot check backup bucket: %w", err)
		}
		bucketExists = (err == nil)
	}

	if bucketExists {
		l.Info("Creating repository password for bucket access")
		err = createRepositoryPassword(ctx, comp, svc)
		if err != nil {
			return fmt.Errorf("cannot create repository password: %w", err)
		}
	}

	// Remove the backup schedule for suspended instances
	if comp.IsBackupEnabled() && comp.GetInstances() != 0 {
		l.Info("Creating backup schedule - backups enabled")
		err = createK8upSchedule(ctx, comp, svc)
		if err != nil {
			return fmt.Errorf("cannot create backup schedule, %w", err)
		}
	} else {
		l.Info("Skipping backup schedule - backups disabled")
	}

	return nil
}

// Create object bucket for backups
func CreateObjectBucket(ctx context.Context, comp common.InfoGetter, svc *runtime.ServiceRuntime) error {
	l := controllerruntime.LoggerFrom(ctx)

	if comp.GetName() == "" {
		return fmt.Errorf("could not get composite name")
	}

	// Start with base labels
	labels := map[string]string{
		runtime.ProviderConfigIgnoreLabel: "true",
	}

	// Check if backup is disabled and we need to preserve an existing bucket for retention
	if !comp.IsBackupEnabled() {
		l.Info("Backup disabled - checking if bucket needs retention timestamp label")

		// Check if bucket exists in observed state (from when backup was enabled)
		observedBucket := &appcatv1.XObjectBucket{}
		bucketName := comp.GetName() + "-backup"
		err := svc.GetObservedComposedResource(observedBucket, bucketName)
		if err == runtime.ErrNotFound {
			l.Info("No existing backup bucket found - backup was never enabled, skipping bucket creation")
			return nil
		}
		if err != nil {
			return fmt.Errorf("cannot check observed backup bucket: %w", err)
		}

		// Bucket exists from when backup was enabled - preserve it with timestamp label
		l.Info("Found existing bucket from when backup was enabled - adding retention timestamp")

		// Copy existing labels
		if observedBucket.Labels != nil {
			for k, v := range observedBucket.Labels {
				labels[k] = v
			}
		}

		// Add backup-disabled timestamp label only if it doesn't exist yet
		if _, exists := labels[BackupDisabledTimestampLabel]; !exists {
			now := time.Now()
			timestampStr := strconv.FormatInt(now.Unix(), 10)
			labels[BackupDisabledTimestampLabel] = timestampStr
			l.Info("Added backup disabled timestamp label to bucket for retention tracking",
				"timestamp", now.Format(time.RFC3339),
				"timestampUnix", timestampStr)
		} else {
			l.Info("Backup disabled timestamp label already exists")

			// Check if retention period has expired and bucket should be deleted
			timestampStr := labels[BackupDisabledTimestampLabel]
			timestampUnix, err := strconv.ParseInt(timestampStr, 10, 64)
			if err != nil {
				l.Error(err, "Failed to parse backup disabled timestamp, skipping retention check")
			} else {
				backupDisabledTime := time.Unix(timestampUnix, 0)
				retention := comp.GetBackupRetention()

				// Use KeepDaily as the retention period in days, with a minimum of 1 day
				retentionDays := retention.KeepDaily
				if retentionDays <= 0 {
					retentionDays = 6 // Default value matching webhook
				}

				retentionPeriod := time.Duration(retentionDays) * 24 * time.Hour
				allowedDeletionTime := backupDisabledTime.Add(retentionPeriod)
				now := time.Now()

				if now.After(allowedDeletionTime) {
					// Retention period has expired, skip bucket creation to allow deletion
					l.Info("Backup bucket retention period has expired, skipping bucket recreation to allow deletion",
						"retentionDays", retentionDays,
						"backupDisabledTime", backupDisabledTime.Format(time.RFC3339),
						"expirationTime", allowedDeletionTime.Format(time.RFC3339))
					return nil
				} else {
					timeRemaining := allowedDeletionTime.Sub(now)
					l.Info("Backup bucket still in retention period, preserving it",
						"retentionDays", retentionDays,
						"backupDisabledTime", backupDisabledTime.Format(time.RFC3339),
						"timeRemaining", timeRemaining.String())
					labels["appcat.vshn.io/allow-deletion"] = "true"
					// Also patch the connection secret with the same label
					if err := PatchConnectionSecretWithAllowDeletion(ctx, comp, svc); err != nil {
						l.Error(err, "Failed to patch connection secret with allow-deletion label")
					}
				}
			}
		}
	}

	ob := &appcatv1.XObjectBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:   comp.GetName() + "-backup",
			Labels: labels,
		},
		Spec: appcatv1.XObjectBucketSpec{
			Parameters: appcatv1.ObjectBucketParameters{
				BucketName: fmt.Sprintf("%s-%s-%s", comp.GetName(), svc.Config.Data["bucketRegion"], "backup"),
			},
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Namespace: svc.GetCrossplaneNamespace(),
					Name:      credentialSecretName + "-" + comp.GetName(),
				},
			},
		},
	}

	ob.Spec.Parameters.BucketName = getBucketName(svc, ob)

	return svc.SetDesiredComposedResource(ob)
}

func createRepositoryPassword(ctx context.Context, comp common.InfoGetter, svc *runtime.ServiceRuntime) error {

	l := controllerruntime.LoggerFrom(ctx)

	secretName := comp.GetName() + "-k8up-repo-pw"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8upRepoSecretName,
			Namespace: comp.GetInstanceNamespace(),
		},
	}

	err := svc.GetObservedKubeObject(secret, secretName)
	if err != nil && err != runtime.ErrNotFound {
		return err
	}

	if _, ok := secret.Data[k8upRepoSecretKey]; ok {
		l.V(1).Info("secret is not empty")
		return svc.SetDesiredKubeObject(secret, secretName, runtime.KubeOptionAllowDeletion)
	}

	pw, err := password.Generate(64, 5, 5, false, true)
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		k8upRepoSecretKey: []byte(pw),
	}

	return svc.SetDesiredKubeObject(secret, secretName, runtime.KubeOptionAllowDeletion)
}

func createK8upSchedule(ctx context.Context, comp common.InfoGetter, svc *runtime.ServiceRuntime) error {

	l := controllerruntime.LoggerFrom(ctx)

	cd, err := svc.GetObservedComposedResourceConnectionDetails(comp.GetName() + "-backup")
	if err != nil && err == runtime.ErrNotFound {
		l.V(1).Info("credential secret not found, skipping schedule")
		return nil
	} else if err != nil {
		return err
	}

	bucket := string(cd["BUCKET_NAME"])
	endpoint := string(cd["ENDPOINT_URL"])
	retention := comp.GetBackupRetention()

	endpoint, _ = strings.CutSuffix(endpoint, "/")

	schedule := &k8upv1.Schedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetServiceName() + "-schedule",
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: k8upv1.ScheduleSpec{
			Backend: &k8upv1.Backend{
				RepoPasswordSecretRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: k8upRepoSecretName,
					},
					Key: k8upRepoSecretKey,
				},
				S3: &k8upv1.S3Spec{
					Endpoint: endpoint,
					Bucket:   bucket,
					AccessKeyIDSecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: credentialSecretName + "-" + comp.GetName(),
						},
						Key: "AWS_ACCESS_KEY_ID",
					},
					SecretAccessKeySecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: credentialSecretName + "-" + comp.GetName(),
						},
						Key: "AWS_SECRET_ACCESS_KEY",
					},
				},
			},
			Backup: &k8upv1.BackupSchedule{
				ScheduleCommon: &k8upv1.ScheduleCommon{
					Schedule: k8upv1.ScheduleDefinition(comp.GetBackupSchedule()),
				},
				BackupSpec: k8upv1.BackupSpec{
					KeepJobs: ptr.To(0),
				},
			},
			Prune: &k8upv1.PruneSchedule{
				ScheduleCommon: &k8upv1.ScheduleCommon{
					Schedule: "@weekly-random",
				},
				PruneSpec: k8upv1.PruneSpec{
					Retention: k8upv1.RetentionPolicy{
						KeepLast:    retention.KeepLast,
						KeepHourly:  retention.KeepHourly,
						KeepDaily:   retention.KeepDaily,
						KeepWeekly:  retention.KeepWeekly,
						KeepMonthly: retention.KeepMonthly,
						KeepYearly:  retention.KeepYearly,
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(schedule, comp.GetName()+"-backup-schedule", runtime.KubeOptionAllowDeletion)
}

// AddPVCAnnotationToValues adds the default exclude annotations to the PVCs via the release values.
func AddPVCAnnotationToValues(valueMap map[string]any, path ...string) error {
	annotations := map[string]interface{}{
		"k8up.io/backup": "false",
	}
	err := unstructured.SetNestedMap(valueMap, annotations, path...)
	if err != nil {
		return fmt.Errorf("cannot set annotations the helm values for key: master.persistence")
	}

	return nil
}

// AddPodAnnotationToValues add the annotations to trigger the pre-backup script via the release values.
func AddPodAnnotationToValues(valueMap map[string]any, scriptName, fileExt string, path ...string) error {
	annotations := map[string]interface{}{
		"k8up.io/backupcommand":  scriptName,
		"k8up.io/file-extension": fileExt,
	}
	err := unstructured.SetNestedMap(valueMap, annotations, path...)
	if err != nil {
		return fmt.Errorf("cannot set annotations the helm values for key: master.podAnnotations")
	}

	return nil
}

// AddBackupCMToValues adds the volume mount for the given configMap to the helm values.
// volumePath and mountPath specify the value path within the values map.
// It will mount the configmap under /scripts in the pod.
func AddBackupCMToValues(values map[string]any, volumePath []string, mountPath []string) error {
	volumes := []interface{}{
		corev1.Volume{
			Name: backupScriptCMName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: backupScriptCMName,
					},
					DefaultMode: ptr.To(int32(0774)),
				},
			},
		},
	}

	err := common.SetNestedObjectValue(values, volumePath, volumes)
	if err != nil {
		return err
	}

	volumeMounts := []interface{}{
		corev1.VolumeMount{
			Name:      backupScriptCMName,
			MountPath: "/scripts",
		},
	}
	err = common.SetNestedObjectValue(values, mountPath, volumeMounts)
	if err != nil {
		return err
	}

	return nil
}

// AddBackupScriptCM will add a configmap containing the given script.
// This can then be used to mount into the resulting pod by whatever means applicable (helm values, pod-definition, etc)
func AddBackupScriptCM(svc *runtime.ServiceRuntime, comp common.Composite, script string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup-script",
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string]string{
			"backup.sh": script,
		},
	}

	return svc.SetDesiredKubeObject(cm, comp.GetName()+"-backup-script")
}

func getBucketName(svc *runtime.ServiceRuntime, currentBucket *appcatv1.XObjectBucket) string {

	bucket := &appcatv1.XObjectBucket{}

	err := svc.GetObservedComposedResource(bucket, currentBucket.GetName())
	if err != nil {
		return currentBucket.Spec.Parameters.BucketName
	}

	// If the found bucket has an empty name, we still return the desired bucketName
	// This avoids race conditions during the provisioning, especially for non-converged setups
	if bucket.Spec.Parameters.BucketName == "" {
		return currentBucket.Spec.Parameters.BucketName
	}

	return bucket.Spec.Parameters.BucketName
}

// patchConnectionSecretWithAllowDeletion patches the connection secret (Kubernetes Object)
// that manages the backup credentials with the allow-deletion label
func PatchConnectionSecretWithAllowDeletion(ctx context.Context, comp common.InfoGetter, svc *runtime.ServiceRuntime) error {
	l := controllerruntime.LoggerFrom(ctx)

	secretObjectName := comp.GetName() + "-backup-" + comp.GetName()

	// Create a Kubernetes Object resource to represent the connection secret
	secretObject := &xkube.Object{}

	// Try to get the existing secret object
	err := svc.GetObservedComposedResource(secretObject, secretObjectName)
	if err != nil {
		if err == runtime.ErrNotFound {
			l.V(1).Info("Connection secret object not found yet, skipping label patch", "secretObjectName", secretObjectName)
			return nil
		}
		return fmt.Errorf("cannot get connection secret object: %w", err)
	}

	l.Info("Patching connection secret object with allow-deletion label", "secretObjectName", secretObjectName)

	return svc.SetDesiredKubeObject(secretObject, secretObjectName,
		runtime.KubeOptionAllowDeletion)
}
