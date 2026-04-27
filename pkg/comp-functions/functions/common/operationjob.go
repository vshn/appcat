package common

import (
	"fmt"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// OperationJob describes a one-off Job emitted by a composition function.
// The Job runs `appcat operation <Subcommand>` w/ the given args. Stable Name
// makes re-emission a server-side-apply no-op until the gate that triggered
// emission flips.
type OperationJob struct {
	// Name = stable Job name. Re-emitting w/ same name = no-op.
	Name string
	// Namespace where Job is created (typically instance namespace).
	Namespace string
	// Subcommand is the cobra subcommand under `appcat operation`.
	Subcommand string
	// Args appended after the subcommand.
	Args []string
	// Env vars passed to the container.
	Env []corev1.EnvVar
	// ServiceAccount used to run the Job pod.
	ServiceAccount string
}

// EmitOperationJob constructs a Kubernetes Job and adds it to the desired
// composed resources via SetDesiredKubeObject. The Job is owned by the XR (via
// the wrapping kube object) so it GCs w/ the instance namespace.
func EmitOperationJob(svc *runtime.ServiceRuntime, op OperationJob) error {
	imageTag := svc.Config.Data["imageTag"]
	if imageTag == "" {
		return fmt.Errorf("no imageTag field in composition function configuration")
	}

	args := append([]string{"operation", op.Subcommand}, op.Args...)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      op.Name,
			Namespace: op.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: ptr.To(int32(3600)),
			BackoffLimit:            ptr.To(int32(3)),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: op.ServiceAccount,
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:  "operation",
							Image: "ghcr.io/vshn/appcat:" + imageTag,
							Args:  args,
							Env:   op.Env,
						},
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(job, op.Name, runtime.KubeOptionAllowDeletion)
}
