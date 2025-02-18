package backup

import (
	"context"
	"fmt"
	"strings"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/sethvargo/go-password/password"
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
	credentialSecretName = "backup-bucket-credentials"
	k8upRepoSecretName   = "k8up-repository-password"
	k8upRepoSecretKey    = "password"
	backupScriptCMName   = "backup-script"
)

// AddK8upBackup creates an S3 bucket and a K8up schedule according to the composition spec.
func AddK8upBackup(ctx context.Context, svc *runtime.ServiceRuntime, comp common.InfoGetter) error {

	l := controllerruntime.LoggerFrom(ctx)

	l.Info("Creating backup bucket")
	err := createObjectBucket(ctx, comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create backup bucket: %w", err)
	}

	l.Info("Creating repository password")
	err = createRepositoryPassword(ctx, comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create repository password: %w", err)
	}

	l.Info("Creating backup schedule")
	err = createK8upSchedule(ctx, comp, svc)
	if err != nil {
		return fmt.Errorf("cannot create backup schedule, %w", err)
	}

	return nil
}

func createObjectBucket(ctx context.Context, comp common.InfoGetter, svc *runtime.ServiceRuntime) error {
	ob := &appcatv1.XObjectBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName() + "-backup",
			Labels: map[string]string{
				runtime.ProviderConfigIgnoreLabel: "true",
			},
		},
		Spec: appcatv1.XObjectBucketSpec{
			Parameters: appcatv1.ObjectBucketParameters{
				BucketName: comp.GetName() + "-backup",
				Region:     svc.Config.Data["bucketRegion"],
			},
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Namespace: svc.GetCrossplaneNamespace(),
					Name:      credentialSecretName,
				},
			},
		},
	}

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
		return svc.SetDesiredKubeObject(secret, secretName)
	}

	pw, err := password.Generate(64, 5, 5, false, true)
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		k8upRepoSecretKey: []byte(pw),
	}

	return svc.SetDesiredKubeObject(secret, secretName)
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
							Name: credentialSecretName,
						},
						Key: "AWS_ACCESS_KEY_ID",
					},
					SecretAccessKeySecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: credentialSecretName,
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

	return svc.SetDesiredKubeObject(schedule, comp.GetName()+"-backup-schedule")
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
