package vshnredis

import (
	"context"
	_ "embed"
	"strings"

	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	runtime "github.com/vshn/appcat/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

//go:embed script/prepRestore.sh
var prepRestoreScript string

//go:embed script/restore.sh
var restoreScript string

//go:embed script/cleanupRestore.sh
var cleanupRestoreScript string

func RestoreBackup(ctx context.Context, iof *runtime.Runtime) runtime.Result {
	log := controllerruntime.LoggerFrom(ctx)
	log.Info("Starting RestoreBackup function")

	comp := &vshnv1.VSHNRedis{}
	err := iof.Observed.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot get composite from function io", err)
	}

	// Wait for the next reconciliation in case instance namespace is missing
	if comp.Status.InstanceNamespace == "" {
		return runtime.NewWarning(ctx, "Composite is missing instance namespace, skipping transformation")
	}

	if comp.Spec.Parameters.Restore.BackupName == "" && comp.Spec.Parameters.Restore.ClaimName == "" {
		return runtime.NewNormal()
	}

	if comp.Spec.Parameters.Restore.ClaimName == "" {
		return runtime.NewWarning(ctx, "Composite is missing claimName parameter to restore from backup")
	}

	if comp.Spec.Parameters.Restore.BackupName == "" {
		return runtime.NewWarning(ctx, "Composite is missing backupName parameter to restore from backup")
	}

	log.Info("Prepare for restore")

	err = addPrepareRestoreJob(ctx, comp, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Can't deploy prepareRestore Job", err)
	}

	log.Info("Restoring from backup")

	err = addRestoreJob(ctx, comp, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Can't deploy restore Job", err)
	}

	log.Info("Cleanup restore")

	err = addCleanUpJob(ctx, comp, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Can't deploy cleanupRestore job", err)
	}

	log.Info("Finishing Restoring RestoreBackup function")

	return runtime.NewNormal()

}

func addPrepareRestoreJob(ctx context.Context, comp *vshnv1.VSHNRedis, iof *runtime.Runtime) error {
	prepRestoreJobName := truncateObjectName(comp.Name + "-" + comp.Spec.Parameters.Restore.BackupName + "-prepare-job")
	claimNamespaceLabel := "crossplane.io/claim-namespace"

	prepJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prepRestoreJobName,
			Namespace: iof.Config.Data["controlNamespace"],
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      "Never",
					ServiceAccountName: iof.Config.Data["restoreSA"],
					Containers: []corev1.Container{
						{
							Name:  "copyjob",
							Image: "bitnami/kubectl:latest",
							Command: []string{
								"bash",
								"-c",
							},
							Args: []string{prepRestoreScript},
							Env: []corev1.EnvVar{
								{
									Name:  "CLAIM_NAMESPACE",
									Value: comp.ObjectMeta.Labels[claimNamespaceLabel],
								},
								{
									Name:  "SOURCE_CLAIM_NAME",
									Value: comp.Spec.Parameters.Restore.ClaimName,
								},
								{
									Name:  "BACKUP_NAME",
									Value: comp.Spec.Parameters.Restore.BackupName,
								},
								{
									Name:  "TARGET_NAMESPACE",
									Value: comp.Status.InstanceNamespace,
								},
							},
						},
					},
				},
			},
		},
	}

	return iof.Desired.PutIntoObject(ctx, prepJob, prepRestoreJobName)
}

func addRestoreJob(ctx context.Context, comp *vshnv1.VSHNRedis, iof *runtime.Runtime) error {
	restoreJobName := truncateObjectName(comp.Name + "-" + comp.Spec.Parameters.Restore.BackupName + "-restore-job")
	restoreSecret := "restore-credentials-" + comp.Spec.Parameters.Restore.BackupName

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restoreJobName,
			Namespace: comp.Status.InstanceNamespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "redis-data-redis-master-0",
								},
							},
						},
					},
					RestartPolicy: "Never",
					Containers: []corev1.Container{
						{
							Name:  "restic",
							Image: "ghcr.io/k8up-io/k8up:master",
							Command: []string{
								"bash",
								"-c",
							},
							Args: []string{restoreScript},
							Env: []corev1.EnvVar{
								{
									Name: "AWS_ACCESS_KEY_ID",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: restoreSecret,
											},
											Key: "AWS_ACCESS_KEY_ID",
										},
									},
								},
								{
									Name: "AWS_SECRET_ACCESS_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: restoreSecret,
											},
											Key: "AWS_SECRET_ACCESS_KEY",
										},
									},
								},
								{
									Name: "RESTIC_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: restoreSecret,
											},
											Key: "RESTIC_PASSWORD",
										},
									},
								},
								{
									Name: "RESTIC_REPOSITORY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: restoreSecret,
											},
											Key: "RESTIC_REPOSITORY",
										},
									},
								},
								{
									Name: "BACKUP_NAME",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: restoreSecret,
											},
											Key: "BACKUP_NAME",
										},
									},
								},
								{
									Name: "BACKUP_PATH",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: restoreSecret,
											},
											Key: "BACKUP_PATH",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/data",
								},
							},
						},
					},
				},
			},
		},
	}

	return iof.Desired.PutIntoObject(ctx, job, restoreJobName)
}

func addCleanUpJob(ctx context.Context, comp *vshnv1.VSHNRedis, iof *runtime.Runtime) error {
	cleanupRestoreJobName := truncateObjectName(comp.Name + "-" + comp.Spec.Parameters.Restore.BackupName + "-cleanup-job")
	restoreJobName := truncateObjectName(comp.Name + "-" + comp.Spec.Parameters.Restore.BackupName + "-restore-job")
	restoreSecret := "statefulset-replicas-" + comp.Spec.Parameters.Restore.ClaimName + "-" + comp.Spec.Parameters.Restore.BackupName
	claimNamespaceLabel := "crossplane.io/claim-namespace"
	claimNameLabel := "crossplane.io/claim-name"

	prepJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cleanupRestoreJobName,
			Namespace: iof.Config.Data["controlNamespace"],
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      "Never",
					ServiceAccountName: iof.Config.Data["restoreSA"],
					Containers: []corev1.Container{
						{
							Name:  "copyjob",
							Image: "bitnami/kubectl:latest",
							Command: []string{
								"bash",
								"-c",
							},
							Args: []string{cleanupRestoreScript},
							Env: []corev1.EnvVar{
								{
									Name:  "CLAIM_NAMESPACE",
									Value: comp.ObjectMeta.Labels[claimNamespaceLabel],
								},
								{
									Name:  "SOURCE_CLAIM_NAME",
									Value: comp.Spec.Parameters.Restore.ClaimName,
								},
								{
									Name:  "DEST_CLAIM_NAME",
									Value: comp.ObjectMeta.Labels[claimNameLabel],
								},
								{
									Name:  "RESTORE_JOB_NAME",
									Value: restoreJobName,
								},
								{
									Name:  "BACKUP_NAME",
									Value: comp.Spec.Parameters.Restore.BackupName,
								},
								{
									Name:  "TARGET_NAMESPACE",
									Value: comp.Status.InstanceNamespace,
								},
								{
									Name: "NUM_REPLICAS",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: restoreSecret,
											},
											Key: "NUM_REPLICAS",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return iof.Desired.PutIntoObject(ctx, prepJob, cleanupRestoreJobName)
}

func truncateObjectName(s string) string {
	if 63 > len(s) {
		return s
	}
	return s[:strings.LastIndex(s[:63], " ")]
}
