package vshnredis

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/sethvargo/go-password/password"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
	credentialSecretName = "backup-bucket-credentials"
	k8upRepoSecretName   = "k8up-repository-password"
	k8upRepoSecretKey    = "password"
	backupScriptCMName   = "backup-script"
)

//go:embed script/backup.sh
var redisBackupScript string

// AddBackup creates an object bucket and a K8up schedule to do the actual backup.
func AddBackup(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	l := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNRedis{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("failed to parse composite: %w", err))
	}

	common.SetRandomSchedules(comp, comp)

	err = svc.SetDesiredCompositeStatus(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("failed to set composite: %w", err))
	}

	l.Info("Creating backup bucket")
	err = createObjectBucket(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create backup bucket: %w", err))
	}

	l.Info("Creating credential observer")
	err = createObjectBucketCredentialObserver(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create credential observer: %w", err))
	}

	l.Info("Creating repository password")
	err = createRepositoryPassword(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create repository password: %w", err))
	}

	l.Info("Creating backup schedule")
	err = createK8upSchedule(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create backup schedule, %w", err))
	}

	l.Info("Creating backup config map")
	err = createScriptCM(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create backup config map: %w", err))
	}

	return nil
}

func createObjectBucket(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) error {

	ob := &appcatv1.XObjectBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.Name + "-backup",
		},
		Spec: appcatv1.XObjectBucketSpec{
			Parameters: appcatv1.ObjectBucketParameters{
				BucketName: comp.Name + "-backup",
				Region:     svc.Config.Data["bucketRegion"],
			},
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Namespace: getInstanceNamespace(comp),
					Name:      credentialSecretName,
				},
			},
		},
	}

	return svc.SetDesiredComposedResource(ob)
}

func createObjectBucketCredentialObserver(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) error {

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
			ResourceSpec: xkube.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: "kubernetes",
				},
			},
		},
	}

	return svc.SetDesiredComposedResource(xobj)
}

func createRepositoryPassword(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) error {

	l := controllerruntime.LoggerFrom(ctx)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8upRepoSecretName,
			Namespace: getInstanceNamespace(comp),
		},
	}

	err := svc.GetObservedKubeObject(secret, comp.Name+"-k8up-repo-pw")
	if err != nil && err != runtime.ErrNotFound {
		return err
	}

	if _, ok := secret.Data[k8upRepoSecretKey]; ok {
		l.V(1).Info("secret is not empty")
		return svc.SetDesiredKubeObject(secret, comp.Name+"-k8up-repo-pw")
	}

	pw, err := password.Generate(64, 5, 5, false, true)
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		k8upRepoSecretKey: []byte(pw),
	}

	return svc.SetDesiredKubeObject(secret, comp.Name+"-k8up-repo-pw")
}

func createK8upSchedule(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) error {

	l := controllerruntime.LoggerFrom(ctx)

	creds := &corev1.Secret{}

	err := svc.GetObservedKubeObject(creds, comp.Name+"-backup-credential-observer")
	if err != nil && err == runtime.ErrNotFound {
		l.V(1).Info("credential secret not found, skipping schedule")
		return nil
	} else if err != nil {
		return err
	}

	bucket := string(creds.Data["BUCKET_NAME"])
	endpoint := string(creds.Data["ENDPOINT_URL"])
	retention := comp.Spec.Parameters.Backup.Retention

	endpoint, _ = strings.CutSuffix(endpoint, "/")

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

	return svc.SetDesiredKubeObject(schedule, comp.Name+"-backup-schedule")
}

func createScriptCM(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) error {

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupScriptCMName,
			Namespace: getInstanceNamespace(comp),
		},
		Data: map[string]string{
			"backup.sh": redisBackupScript,
		},
	}

	return svc.SetDesiredKubeObject(cm, comp.Name+"-backup-cm")
}
