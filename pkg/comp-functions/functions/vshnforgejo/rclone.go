package vshnforgejo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/sethvargo/go-password/password"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
	rcloneCredentialsSecretName = "rclone-credentials"
	k8upRepoSecretName          = "k8up-repository-password"
	k8upRepoSecretKey           = "password"
	credentialSecretName        = "backup-bucket-credentials"
)

// AddRcloneBackup creates an rclone deployment with S3 compatibility and configures k8up to use it
func AddRcloneBackup(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) error {
	l := controllerruntime.LoggerFrom(ctx)

	// Check if backups are enabled
	if !comp.IsBackupEnabled() {
		l.Info("Backups disabled, skipping rclone deployment")
		return nil
	}

	// Get resources for sizing
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])
	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		return fmt.Errorf("could not fetch plans: %w", err)
	}
	res, errs := common.GetResources(&comp.Spec.Parameters.Size, resources)
	if len(errs) > 0 {
		svc.Log.Error(fmt.Errorf("could not get resources"), "errors", errors.Join(errs...))
	}

	// Create credentials first
	l.Info("Creating rclone credentials")
	err = createRcloneCredentials(comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create rclone credentials: %w", err)
	}

	// Create storage
	l.Info("Creating rclone PVC")
	err = createRclonePVC(comp, svc, res)
	if err != nil {
		return fmt.Errorf("cannot create rclone PVC: %w", err)
	}

	// Create deployment
	l.Info("Creating rclone deployment")
	err = createRcloneDeployment(comp, svc, res)
	if err != nil {
		return fmt.Errorf("cannot create rclone deployment: %w", err)
	}

	// Create service
	l.Info("Creating rclone service")
	err = createRcloneService(comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create rclone service: %w", err)
	}

	// Create k8up connection secret
	l.Info("Creating k8up connection secret")
	err = createK8upConnectionSecret(comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create k8up connection secret: %w", err)
	}

	// Create k8up repository password
	l.Info("Creating k8up repository password")
	err = createRepositoryPassword(ctx, comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create k8up repo password: %w", err)
	}

	// Create k8up schedule (only if instances > 0)
	if comp.GetInstances() > 0 {
		l.Info("Creating k8up schedule")
		err = createK8upScheduleForRclone(ctx, comp, svc)
		if err != nil {
			return fmt.Errorf("cannot create k8up schedule: %w", err)
		}
	} else {
		l.Info("Instance suspended, skipping k8up schedule")
	}

	return nil
}

// createRcloneCredentials generates random S3 credentials for rclone
func createRcloneCredentials(comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) error {
	secretName := comp.GetName() + "-rclone-credentials"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rcloneCredentialsSecretName,
			Namespace: comp.GetInstanceNamespace(),
		},
	}

	err := svc.GetObservedKubeObject(secret, secretName)
	if err != nil && err != runtime.ErrNotFound {
		return err
	}

	// Check if secret already exists with data
	if _, ok := secret.Data["access_key"]; ok {
		svc.Log.V(1).Info("rclone credentials already exist, reusing")
		return svc.SetDesiredKubeObject(secret, secretName, runtime.KubeOptionAllowDeletion)
	}

	// Generate new credentials
	accessKey, err := password.Generate(20, 5, 0, false, true)
	if err != nil {
		return fmt.Errorf("cannot generate access key: %w", err)
	}
	secretKey, err := password.Generate(40, 10, 0, false, true)
	if err != nil {
		return fmt.Errorf("cannot generate secret key: %w", err)
	}

	secret.Data = map[string][]byte{
		"access_key": []byte(accessKey),
		"secret_key": []byte(secretKey),
	}

	return svc.SetDesiredKubeObject(secret, secretName, runtime.KubeOptionAllowDeletion)
}

// createRclonePVC creates a dedicated PVC for rclone backup storage
func createRclonePVC(comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime, resources common.Resources) error {
	pvcName := comp.GetName() + "-rclone-pvc"

	// Use a fixed 10Gi size for POC - backups are typically smaller than the source data
	backupStorageSize := resource.MustParse("10Gi")

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-rclone-storage",
			Namespace: comp.GetInstanceNamespace(),
			Labels: map[string]string{
				"app": comp.GetName() + "-rclone",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: backupStorageSize,
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(pvc, pvcName, runtime.KubeOptionAllowDeletion)
}

// createRcloneDeployment creates the rclone serve s3 Deployment
func createRcloneDeployment(comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime, resources common.Resources) error {
	deploymentName := comp.GetName() + "-rclone-deployment"

	replicas := int32(1)
	if comp.GetInstances() == 0 {
		replicas = 0
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-rclone",
			Namespace: comp.GetInstanceNamespace(),
			Labels: map[string]string{
				"app": comp.GetName() + "-rclone",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": comp.GetName() + "-rclone"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: comp.GetName() + "-rclone",
					Labels: map[string]string{
						"app": comp.GetName() + "-rclone",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "rclone",
							Image: "rclone/rclone:latest",
							Command: []string{
								"rclone",
								"serve",
								"s3",
								"/data",
								"--addr",
								":8080",
								"--auth-key",
								"$(ACCESS_KEY),$(SECRET_KEY)",
								"--no-checksum",
								"--no-modtime",
								"--vfs-cache-mode",
								"full",
								"--vfs-write-back",
								"0s",
								"--dir-cache-time",
								"10s",
								"--force-path-style=true",
							},
							Env: []corev1.EnvVar{
								{
									Name: "ACCESS_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: rcloneCredentialsSecretName,
											},
											Key: "access_key",
										},
									},
								},
								{
									Name: "SECRET_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: rcloneCredentialsSecretName,
											},
											Key: "secret_key",
										},
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
									Name:          "s3",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "rclone-data",
									MountPath: "/data",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "rclone-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: comp.GetName() + "-rclone-storage",
								},
							},
						},
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(deployment, deploymentName, runtime.KubeOptionAllowDeletion)
}

// createRcloneService creates a Service exposing rclone on port 8080
func createRcloneService(comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) error {
	serviceName := comp.GetName() + "-rclone-service"

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-rclone",
			Namespace: comp.GetInstanceNamespace(),
			Labels: map[string]string{
				"app": comp.GetName() + "-rclone",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": comp.GetName() + "-rclone",
			},
			Ports: []corev1.ServicePort{
				{
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
					Name:       "s3",
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(service, serviceName, runtime.KubeOptionAllowDeletion)
}

// createK8upConnectionSecret creates the S3 connection secret that k8up will use
func createK8upConnectionSecret(comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) error {
	secretName := comp.GetName() + "-backup-connection"

	// Read rclone credentials
	rcloneCreds := &corev1.Secret{}
	err := svc.GetObservedKubeObject(rcloneCreds, comp.GetName()+"-rclone-credentials")
	if err != nil {
		return fmt.Errorf("cannot get rclone credentials: %w", err)
	}

	// Ensure credentials exist
	accessKey, ok := rcloneCreds.Data["access_key"]
	if !ok {
		return fmt.Errorf("rclone credentials missing access_key")
	}
	secretKey, ok := rcloneCreds.Data["secret_key"]
	if !ok {
		return fmt.Errorf("rclone credentials missing secret_key")
	}

	// Create endpoint URL
	endpoint := fmt.Sprintf("http://%s-rclone.%s.svc.cluster.local:8080",
		comp.GetName(), comp.GetInstanceNamespace())

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      credentialSecretName + "-" + comp.GetName(),
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string][]byte{
			"AWS_ACCESS_KEY_ID":     accessKey,
			"AWS_SECRET_ACCESS_KEY": secretKey,
			"ENDPOINT_URL":          []byte(endpoint),
			"BUCKET_NAME":           []byte("backup"),
			"AWS_REGION":            []byte("us-east-1"),
		},
	}

	return svc.SetDesiredKubeObject(secret, secretName, runtime.KubeOptionAllowDeletion)
}

// createRepositoryPassword creates the k8up repository password secret
func createRepositoryPassword(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) error {
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
		l.V(1).Info("k8up repository password already exists")
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

// createK8upScheduleForRclone creates the k8up Schedule pointing to the rclone backend
func createK8upScheduleForRclone(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) error {
	l := controllerruntime.LoggerFrom(ctx)

	// Get connection details for endpoint
	cd := make(map[string][]byte)
	connectionSecret := &corev1.Secret{}
	err := svc.GetObservedKubeObject(connectionSecret, comp.GetName()+"-backup-connection")
	if err != nil && err == runtime.ErrNotFound {
		l.V(1).Info("connection secret not found, skipping schedule")
		return nil
	} else if err != nil {
		return err
	}
	cd = connectionSecret.Data

	bucket := string(cd["BUCKET_NAME"])
	endpoint := string(cd["ENDPOINT_URL"])
	retention := comp.GetBackupRetention()

	endpoint, _ = strings.CutSuffix(endpoint, "/")

	scheduleName := comp.GetName() + "-backup-schedule"

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

	return svc.SetDesiredKubeObject(schedule, scheduleName, runtime.KubeOptionAllowDeletion)
}
