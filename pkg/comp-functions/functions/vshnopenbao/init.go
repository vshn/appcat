package vshnopenbao

import (
	"context"
	_ "embed"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

//go:embed scripts/init_cluster.sh
var initClusterScript string

// InitOpenBao creates a one-shot Job that initializes the OpenBao Raft cluster on first boot.
// Once initialization is complete the Job completes and its pod terminates. On subsequent
// reconciles the Job is omitted from desired state once the init-output secret is observed,
// causing provider-kubernetes to clean it up.
func InitOpenBao(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	serviceName := comp.GetName()
	ns := comp.GetInstanceNamespace()

	image := svc.Config.Data["openbao_image"]
	if image == "" {
		return runtime.NewFatalResult(fmt.Errorf("openbao_image is not set in the composition config"))
	}

	// If the init-output secret has already been observed, initialization is done.
	initSecret := &corev1.Secret{}
	observerName := serviceName + initOutputSecretSuffix + observerSuffix
	if err := svc.GetObservedKubeObject(initSecret, observerName); err == nil && len(initSecret.Data) > 0 {
		return nil
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "-init",
			Namespace: ns,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To[int32](1_000_000),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceName,
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    "init-openbao",
							Image:   image,
							Command: []string{"sh", "-c"},
							Args:    []string{initClusterScript},
							Env: []corev1.EnvVar{
								{Name: "POD_NAME", Value: serviceName + "-0"},
								{Name: "VAULT_ADDR", Value: fmt.Sprintf("https://%s:8200", serviceName)},
								// Headless pod address: regular service only routes to ready pods,
								// but pod-0 is not ready until after initialization.
								{Name: "VAULT_INIT_ADDR", Value: fmt.Sprintf("https://%s-0.%s-internal.%s.svc.cluster.local:8200", serviceName, serviceName, ns)},
								{Name: "NAMESPACE", Value: ns},
								{Name: "ROOT_TOKEN_SECRET_NAME", Value: serviceName + initOutputSecretSuffix},
								{Name: "UNSEAL_KEYS_SECRET_NAME", Value: serviceName + unsealKeysSecretSuffix},
								{Name: "SECRET_SHARES", Value: "5"},
								{Name: "SECRET_THRESHOLD", Value: "3"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      hclConfigTlsVolumeName,
									MountPath: "/tls",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: hclConfigTlsVolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									DefaultMode: ptr.To[int32](420),
									SecretName:  serverCertSecretName,
								},
							},
						},
					},
				},
			},
		},
	}

	if err := svc.SetDesiredKubeObject(job, serviceName+"-init", runtime.KubeOptionAllowDeletion); err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot add init job: %w", err).Error())
	}
	return nil
}
