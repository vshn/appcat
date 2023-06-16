package vshnredis

import (
	"context"
	_ "embed"
	"encoding/json"

	xhelm "github.com/crossplane-contrib/provider-helm/apis/release/v1beta1"
	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/sethvargo/go-password/password"
	appcatv1 "github.com/vshn/appcat/apis/v1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
	credentialSecretName = "backup-bucket-credentials"
	k8upRepoSecretName   = "k8up-repository-password"
	k8upRepoSecretKey    = "password"
	backupScriptCMName   = "backup-script"
)

type backupResFunc func(ctx context.Context, comp *vshnv1.VSHNRedis, iof *runtime.Runtime) error

type backupCompStep struct {
	Name    string
	ResFunc backupResFunc
}

var backupResources = []backupCompStep{
	{
		Name:    "objectbucket",
		ResFunc: createObjectBucket,
	},
	{
		Name:    "credential observer",
		ResFunc: createObjectBucketCredentialObserver,
	},
	{
		Name:    "repository password",
		ResFunc: createRepositoryPassword,
	},
	{
		Name:    "k8up schedule",
		ResFunc: createK8upSchedule,
	},
	{
		Name:    "backup cm",
		ResFunc: createScriptCM,
	},
	{
		Name:    "adjust helm values",
		ResFunc: adjustHelmValues,
	},
}

//go:embed script/backup.sh
var redisBackupScript string

// AddBackup creates an object bucket and a K8up schedule to do the actual backup.
func AddBackup(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	l := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNRedis{}
	err := iof.Desired.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "failed to parse composite", err)
	}

	common.SetRandomSchedules(&comp.Spec.Parameters.Backup, &comp.Spec.Parameters.Maintenance)

	err = iof.Desired.SetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "failed to set composite", err)
	}

	for _, res := range backupResources {
		l.Info("running backup step", "name", res.Name)
		err := res.ResFunc(ctx, comp, iof)
		if err != nil {
			return runtime.NewFatalErr(ctx, "error during backup step: "+res.Name, err)
		}
	}

	err = createScriptCM(ctx, comp, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot add backup cm", err)
	}

	return runtime.NewNormal()
}

func createObjectBucket(ctx context.Context, comp *vshnv1.VSHNRedis, iof *runtime.Runtime) error {

	ob := &appcatv1.XObjectBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.Name + "-backup",
		},
		Spec: appcatv1.XObjectBucketSpec{
			Parameters: appcatv1.ObjectBucketParameters{
				BucketName: comp.Name + "-backup",
				Region:     iof.Config.Data["bucketRegion"],
			},
			WriteConnectionSecretToRef: appcatv1.NamespacedName{
				Namespace: getInstanceNamespace(comp),
				Name:      credentialSecretName,
			},
		},
	}

	return iof.Desired.Put(ctx, ob)
}

func createObjectBucketCredentialObserver(ctx context.Context, comp *vshnv1.VSHNRedis, iof *runtime.Runtime) error {

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      credentialSecretName,
			Namespace: getInstanceNamespace(comp),
		},
	}

	xobj := &xkube.Object{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.Name + "-backup-credential-observer",
		},
		Spec: xkube.ObjectSpec{
			ManagementPolicy: xkube.Observe,
			ForProvider: xkube.ObjectParameters{
				Manifest: k8sruntime.RawExtension{
					Object: secret,
				},
			},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: "kubernetes",
				},
			},
		},
	}

	return iof.Desired.Put(ctx, xobj)
}

func createRepositoryPassword(ctx context.Context, comp *vshnv1.VSHNRedis, iof *runtime.Runtime) error {

	l := controllerruntime.LoggerFrom(ctx)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8upRepoSecretName,
			Namespace: getInstanceNamespace(comp),
		},
	}

	err := iof.Observed.GetFromObject(ctx, secret, comp.Name+"-k8up-repo-pw")
	if err != nil && err != runtime.ErrNotFound {
		return err
	}

	if _, ok := secret.Data[k8upRepoSecretKey]; ok {
		l.V(1).Info("secret is not empty")
		return iof.Desired.PutIntoObject(ctx, secret, comp.Name+"-k8up-repo-pw")
	}

	pw, err := password.Generate(64, 5, 5, false, true)
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		k8upRepoSecretKey: []byte(pw),
	}

	return iof.Desired.PutIntoObject(ctx, secret, comp.Name+"-k8up-repo-pw")
}

func createK8upSchedule(ctx context.Context, comp *vshnv1.VSHNRedis, iof *runtime.Runtime) error {

	l := controllerruntime.LoggerFrom(ctx)

	creds := &corev1.Secret{}

	err := iof.Observed.GetFromObject(ctx, creds, comp.Name+"-backup-credential-observer")
	if err != nil && err == runtime.ErrNotFound {
		l.V(1).Info("credential secret not found, skipping schedule")
		return nil
	} else if err != nil {
		return err
	}

	bucket := string(creds.Data["BUCKET_NAME"])
	endpoint := string(creds.Data["ENDPOINT_URL"])
	retention := comp.Spec.Parameters.Backup.Retention

	schedule := &k8upv1.Schedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-schedule",
			Namespace: getInstanceNamespace(comp),
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
					Schedule: k8upv1.ScheduleDefinition(comp.Spec.Parameters.Backup.Schedule),
				},
				BackupSpec: k8upv1.BackupSpec{
					KeepJobs: pointer.Int(0),
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

	return iof.Desired.PutIntoObject(ctx, schedule, comp.Name+"-backup-schedule")
}

func adjustHelmValues(ctx context.Context, comp *vshnv1.VSHNRedis, iof *runtime.Runtime) error {

	l := controllerruntime.LoggerFrom(ctx)

	release := &xhelm.Release{}

	err := iof.Desired.Get(ctx, release, "release")
	if err != nil {
		return err
	}

	valueMap := map[string]any{}

	err = yaml.Unmarshal(release.Spec.ForProvider.Values.Raw, &valueMap)
	if err != nil {
		return err
	}

	valueMap = addPVCAnnotation(valueMap)
	valueMap = addPodAnnotation(valueMap)
	valueMap = addBackupCM(valueMap)

	l.V(1).Info("Final value map", "map", valueMap)

	valueRaw, err := json.Marshal(valueMap)
	if err != nil {
		return err
	}

	release.Spec.ForProvider.Values.Raw = valueRaw

	return iof.Desired.PutWithResourceName(ctx, release, "release")
}

func addPVCAnnotation(valueMap map[string]any) map[string]any {
	persistenceMap := valueMap["master"].(map[string]any)["persistence"].(map[string]any)

	persistenceMap["annotations"] = map[string]string{
		"k8up.io/backup": "false",
	}

	valueMap["master"].(map[string]any)["persistence"] = persistenceMap
	return valueMap
}

func addPodAnnotation(valueMap map[string]any) map[string]any {
	masterMap := valueMap["master"].(map[string]any)

	masterMap["podAnnotations"] = map[string]string{
		"k8up.io/backupcommand":  "/scripts/backup.sh",
		"k8up.io/file-extension": ".tar",
	}

	valueMap["master"] = masterMap
	return valueMap

}

func addBackupCM(valueMap map[string]any) map[string]any {

	masterMap := valueMap["master"].(map[string]any)

	volumes := []corev1.Volume{
		{
			Name: backupScriptCMName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: backupScriptCMName,
					},
					DefaultMode: pointer.Int32(0774),
				},
			},
		},
	}

	masterMap["extraVolumes"] = volumes

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      backupScriptCMName,
			MountPath: "/scripts",
		},
	}

	masterMap["extraVolumeMounts"] = volumeMounts

	valueMap["master"] = masterMap

	return valueMap
}

func createScriptCM(ctx context.Context, comp *vshnv1.VSHNRedis, iof *runtime.Runtime) error {

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupScriptCMName,
			Namespace: getInstanceNamespace(comp),
		},
		Data: map[string]string{
			"backup.sh": redisBackupScript,
		},
	}

	return iof.Desired.PutIntoObject(ctx, cm, comp.Name+"-backup-cm")
}
