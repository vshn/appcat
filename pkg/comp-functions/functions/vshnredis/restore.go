package vshnredis

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	runtime "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
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

func RestoreBackup(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)
	log.Info("Starting RestoreBackup function")

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite from function io: %w", err))
	}

	if comp.Spec.Parameters.Restore.BackupName == "" && comp.Spec.Parameters.Restore.ClaimName == "" {
		return runtime.NewWarningResult("Composite is missing backupName or claimName namespace, skipping transformation")
	}

	if comp.Spec.Parameters.Restore.ClaimName == "" {
		return runtime.NewFatalResult(fmt.Errorf("Composite is missing claimName parameter to restore from backup"))
	}

	if comp.Spec.Parameters.Restore.BackupName == "" {
		return runtime.NewFatalResult(fmt.Errorf("Composite is missing backupName parameter to restore from backup"))
	}

	log.Info("Prepare for restore")

	err = addPrepareRestoreJob(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Can't deploy prepareRestore Job: %w", err))
	}

	log.Info("Restoring from backup")

	err = addRestoreJob(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Can't deploy restore Job: %w", err))
	}

	log.Info("Cleanup restore")

	err = addCleanUpJob(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Can't deploy cleanupRestore job: %w", err))
	}

	log.Info("Finishing Restoring RestoreBackup function")

	return nil

}

func addPrepareRestoreJob(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) error {
	prepRestoreJobName := truncateObjectName(comp.Name + "-" + comp.Spec.Parameters.Restore.BackupName + "-prepare-job")
	claimNamespaceLabel := "crossplane.io/claim-namespace"

	prepJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prepRestoreJobName,
			Namespace: svc.Config.Data["controlNamespace"],
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      "Never",
					ServiceAccountName: svc.Config.Data["restoreSA"],
					Containers: []corev1.Container{
						{
							Name:  "copyjob",
							Image: svc.Config.Data["kubectl_image"],
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
									Value: getInstanceNamespace(comp),
								},
							},
						},
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(prepJob, prepRestoreJobName)
}

func addRestoreJob(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) error {
	restoreJobName := truncateObjectName(comp.Name + "-" + comp.Spec.Parameters.Restore.BackupName + "-restore-job")
	restoreSecret := "restore-credentials-" + comp.Spec.Parameters.Restore.BackupName

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restoreJobName,
			Namespace: getInstanceNamespace(comp),
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

	return svc.SetDesiredKubeObject(job, restoreJobName)
}

func addCleanUpJob(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) error {
	cleanupRestoreJobName := truncateObjectName(comp.Name + "-" + comp.Spec.Parameters.Restore.BackupName + "-cleanup-job")
	restoreJobName := truncateObjectName(comp.Name + "-" + comp.Spec.Parameters.Restore.BackupName + "-restore-job")
	restoreSecret := "statefulset-replicas-" + comp.Spec.Parameters.Restore.ClaimName + "-" + comp.Spec.Parameters.Restore.BackupName
	claimNamespaceLabel := "crossplane.io/claim-namespace"
	claimNameLabel := "crossplane.io/claim-name"

	prepJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cleanupRestoreJobName,
			Namespace: svc.Config.Data["controlNamespace"],
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      "Never",
					ServiceAccountName: svc.Config.Data["restoreSA"],
					Containers: []corev1.Container{
						{
							Name:  "copyjob",
							Image: svc.Config.Data["kubectl_image"],
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
									Value: getInstanceNamespace(comp),
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

	return svc.SetDesiredKubeObject(prepJob, cleanupRestoreJobName)
}

func truncateObjectName(s string) string {
	if 63 > len(s) {
		return s
	}
	return s[:strings.LastIndex(s[:63], " ")]
}
