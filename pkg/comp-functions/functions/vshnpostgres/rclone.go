package vshnpostgres

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-password/password"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
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
	credentialSecretName        = "pgbucket"
)

// AddRcloneBackup creates an rclone deployment with S3 compatibility for PostgreSQL backups
func AddRcloneBackup(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
	l := controllerruntime.LoggerFrom(ctx)

	// Check if backups are enabled
	if !comp.Spec.Parameters.Backup.IsEnabled() {
		l.Info("Backups disabled, skipping rclone deployment")
		return nil
	}

	// Create credentials first
	l.Info("Creating rclone credentials")
	err := createRcloneCredentials(comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create rclone credentials: %w", err)
	}

	// Create storage
	l.Info("Creating rclone PVC")
	err = createRclonePVC(comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create rclone PVC: %w", err)
	}

	// Create deployment
	l.Info("Creating rclone deployment")
	err = createRcloneDeployment(comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create rclone deployment: %w", err)
	}

	// Create service
	l.Info("Creating rclone service")
	err = createRcloneService(comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create rclone service: %w", err)
	}

	// Create StackGres connection secret (formatted for SGObjectStorage)
	l.Info("Creating StackGres connection secret")
	err = createStackGresConnectionSecret(comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create StackGres connection secret: %w", err)
	}

	return nil
}

// createRcloneCredentials generates random S3 credentials for rclone
func createRcloneCredentials(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
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
func createRclonePVC(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
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
func createRcloneDeployment(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
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
								"sh",
								"-c",
								"mkdir -p /data/postgres-backup && rclone serve s3 /data --addr :8080 --auth-key $(ACCESS_KEY),$(SECRET_KEY) --no-checksum --no-modtime --vfs-cache-mode full --vfs-write-back 0s --dir-cache-time 10s --force-path-style=true",
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
func createRcloneService(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
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

// createStackGresConnectionSecret creates the S3 connection secret that StackGres will use
// This secret is formatted to match what SGObjectStorage expects
func createStackGresConnectionSecret(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
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

	// Create endpoint URL pointing to rclone service
	endpoint := fmt.Sprintf("http://%s-rclone.%s.svc.cluster.local:8080",
		comp.GetName(), comp.GetInstanceNamespace())

	// Create secret in the format expected by SGObjectStorage
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      credentialSecretName + "-" + comp.GetName(),
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string][]byte{
			"AWS_ACCESS_KEY_ID":     accessKey,
			"AWS_SECRET_ACCESS_KEY": secretKey,
			"ENDPOINT_URL":          []byte(endpoint),
			"BUCKET_NAME":           []byte("postgres-backup"),
			"AWS_REGION":            []byte("us-east-1"),
		},
	}

	return svc.SetDesiredKubeObject(secret, secretName, runtime.KubeOptionAllowDeletion)
}
