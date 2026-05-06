package vshnopenbao

import (
	"context"
	_ "embed"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

//go:embed scripts/init_cluster.sh
var initClusterScript string

const (
	initSuffix             = "openbao-init"
	initOutputSecretSuffix = "-init-output"
	unsealKeysSecretSuffix = "-unseal-keys"
	initJobSuffix          = "-init-job"
	observerSuffix         = "-observer"
)

func InitializeCluster(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	if !comp.Spec.Parameters.OpenBao.Init.RunInitJob {
		svc.Log.Info("RunInitJob is set to false. Skip initialize process...")
		return nil
	}

	// Always observe secrets so connection details keep flowing to the user's secret.
	err = observeInitConnectionDetails(comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot observe init connection details: %w", err))
	}

	if comp.Status.InitializationComplete {
		svc.Log.Info("OpenBao already initialized, skipping init job")
		return nil
	}

	if isInitSecretPopulated(comp, svc) {
		svc.Log.Info("Init secret detected, marking initialization as complete")
		comp.Status.InitializationComplete = true
		if err = svc.SetDesiredCompositeStatus(comp); err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot set initialization status: %w", err))
		}
		return nil
	}

	err = createSA(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create init RBAC: %w", err))
	}

	err = createInitJob(comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create init job: %w", err))
	}

	return nil
}

func isInitSecretPopulated(comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) bool {
	secret := &corev1.Secret{}
	err := svc.GetObservedKubeObject(secret, comp.GetName()+initOutputSecretSuffix+observerSuffix)
	if err != nil {
		return false
	}
	return len(secret.Data["VAULT_TOKEN"]) > 0
}

func createSA(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) error {
	svc.Log.Info("Create RBAC for OpenBao init job")
	return common.AddSaWithRole(ctx, svc, []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"create", "get", "update", "patch"},
		},
	}, comp.GetName(), comp.GetInstanceNamespace(), initSuffix, true)
}

func createInitJob(comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) error {
	serviceName := comp.GetName()
	ns := comp.GetInstanceNamespace()

	secretShares := comp.Spec.Parameters.OpenBao.Init.SecretShares
	secretThreshold := comp.Spec.Parameters.OpenBao.Init.SecretThreshold
	rootTokenSecretName := serviceName + initOutputSecretSuffix
	unsealKeysSecretName := serviceName + unsealKeysSecretSuffix
	jobName := serviceName + initJobSuffix
	openbaoAddr := fmt.Sprintf("https://%s:8200", serviceName)

	svc.Log.Info("Creating OpenBao init job")
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ns,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            ptr.To(int32(100)),
			TTLSecondsAfterFinished: ptr.To(int32(3600)),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: "sa-" + initSuffix,
					Containers: []corev1.Container{
						{
							Name:    "init-openbao",
							Image:   svc.Config.Data["kubectl_image"],
							Command: []string{"bash", "-c"},
							Args:    []string{initClusterScript},
							Env: []corev1.EnvVar{
								{Name: "VAULT_ADDR", Value: openbaoAddr},
								{Name: "NAMESPACE", Value: ns},
								{Name: "ROOT_TOKEN_SECRET_NAME", Value: rootTokenSecretName},
								{Name: "UNSEAL_KEYS_SECRET_NAME", Value: unsealKeysSecretName},
								{Name: "SECRET_SHARES", Value: fmt.Sprintf("%d", secretShares)},
								{Name: "SECRET_THRESHOLD", Value: fmt.Sprintf("%d", secretThreshold)},
							},
						},
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(job, jobName, runtime.KubeOptionAllowDeletion)
}

func observeInitConnectionDetails(comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) error {
	serviceName := comp.GetName()
	ns := comp.GetInstanceNamespace()

	rootTokenSecretName := serviceName + initOutputSecretSuffix
	unsealKeysSecretName := serviceName + unsealKeysSecretSuffix

	rootTokenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rootTokenSecretName,
			Namespace: ns,
		},
	}
	err := svc.SetDesiredKubeObject(rootTokenSecret, rootTokenSecretName+observerSuffix,
		runtime.KubeOptionObserve,
		runtime.KubeOptionAllowDeletion,
		runtime.KubeOptionAddConnectionDetails(svc.GetCrossplaneNamespace(),
			xkube.ConnectionDetail{
				ObjectReference: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  ns,
					Name:       rootTokenSecretName,
					FieldPath:  "data.VAULT_ADDR",
				},
				ToConnectionSecretKey: "VAULT_ADDR",
			},
			xkube.ConnectionDetail{
				ObjectReference: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  ns,
					Name:       rootTokenSecretName,
					FieldPath:  "data.VAULT_TOKEN",
				},
				ToConnectionSecretKey: "VAULT_TOKEN",
			},
		),
	)
	if err != nil {
		return fmt.Errorf("cannot set root token secret observer: %w", err)
	}

	if err = svc.AddObservedConnectionDetails(rootTokenSecretName + observerSuffix); err != nil {
		return fmt.Errorf("cannot add root token connection details: %w", err)
	}

	unsealKeysSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      unsealKeysSecretName,
			Namespace: ns,
		},
	}
	err = svc.SetDesiredKubeObject(unsealKeysSecret, unsealKeysSecretName+observerSuffix,
		runtime.KubeOptionObserve,
		runtime.KubeOptionAllowDeletion,
		runtime.KubeOptionAddConnectionDetails(svc.GetCrossplaneNamespace(),
			xkube.ConnectionDetail{
				ObjectReference: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  ns,
					Name:       unsealKeysSecretName,
					FieldPath:  "data.keys",
				},
				ToConnectionSecretKey: "keys",
			},
		),
	)
	if err != nil {
		return fmt.Errorf("cannot set unseal keys secret observer: %w", err)
	}

	if err = svc.AddObservedConnectionDetails(unsealKeysSecretName + observerSuffix); err != nil {
		return fmt.Errorf("cannot add unseal keys connection details: %w", err)
	}

	return nil
}
