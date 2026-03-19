package vshnopenbao

import (
	"context"
	_ "embed"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

//go:embed script/init_cluster.sh
var initClusterScript string

const initSuffix = "openbao-init"

func InitializeOpenBaoCluster(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	svc.Log.Info("Creating RBAC for OpenBao init job")
	err = common.AddSaWithRole(ctx, svc, []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"create", "get", "update", "patch"},
		},
	}, comp.GetName(), comp.GetInstanceNamespace(), initSuffix, true)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create init RBAC: %w", err))
	}

	svc.Log.Info("Creating OpenBao init job")
	err = createInitJob(comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create init job: %w", err))
	}

	return nil
}

func createInitJob(comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) error {
	serviceName := comp.GetName()
	ns := comp.GetInstanceNamespace()

	autoUnseal := "false"
	if comp.Spec.Parameters.Service.OpenBaoSettings.AutoUnseal.Enabled {
		autoUnseal = "true"
	}

	initSecretName := serviceName + "-init-output"
	jobName := serviceName + "-init-job"
	openbaoAddr := fmt.Sprintf("https://%s:8200", serviceName)

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
								{Name: "OPENBAO_ADDR", Value: openbaoAddr},
								{Name: "INSTANCE_NAMESPACE", Value: ns},
								{Name: "INIT_SECRET_NAME", Value: initSecretName},
								{Name: "AUTO_UNSEAL", Value: autoUnseal},
							},
						},
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(job, jobName, runtime.KubeOptionAllowDeletion)
}
