package vshnredis

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	helmv1beta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	xkubev1 "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//go:embed script/recreate.sh
var stsRecreateScript string

// ResizePVCs will add a job to do the pvc resize for the instance
func ResizePVCs(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("failed to parse composite: %w", err))
	}

	if comp.Spec.Parameters.Size.Disk == "" {
		return nil
	}

	release, err := getObservedOrDesiredRelease(svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get release: %w", err))
	}

	values, err := common.GetReleaseValues(release)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot parse release values: %w", err))
	}

	err = addStsObserver(svc, comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot observe sts: %w", err))
	}

	patch, result := needReleasePatch(comp, values)
	if result != nil {
		return result
	}

	if patch {
		err = addDeletionJob(svc, comp)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot create RBAC for the deletion job: %w", err))
		}

		return nil
	}

	xJob := &xkubev1.Object{}
	err = svc.GetObservedComposedResource(xJob, comp.Name+"-sts-deleter")
	if err != nil && err != runtime.ErrNotFound {
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed deletion job: %w", err))
	}
	// If there's no job observed, we're done here.
	if err == runtime.ErrNotFound {
		return nil
	}

	sts := &appsv1.StatefulSet{}
	err = svc.GetObservedKubeObject(sts, comp.Name+"-sts-observer")
	if err != nil && err != runtime.ErrNotFound {
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed statefulset job: %w", err))
	}

	// If the xkube object has been created it's still possible that the actual job hasn't been observedJob.
	observedJob := len(xJob.Status.AtProvider.Manifest.Raw) > 0

	// Check the sts if it has been updated
	stsSize := int64(0)
	if len(sts.Spec.VolumeClaimTemplates) > 0 {
		stsSize, _ = sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().AsInt64()
	}
	desiredSize, err := getSizeAsInt(comp.Spec.Parameters.Size.Disk)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot parse desired size: %w", err))
	}
	stsUpdated := stsSize == desiredSize

	deletionJob := &batchv1.Job{}
	if observedJob {
		err := json.Unmarshal(xJob.Status.AtProvider.Manifest.Raw, deletionJob)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot unmarshal sts deleter job: %w", err))
		}
	}

	// The job hasn't been observed yet, so we need to keep it in desired, or we will have a recreate loop
	// Also as long as it hasn't finished we need to make sure it exists.
	if (!observedJob || deletionJob.Status.Succeeded < 1) || (sts.Status.ReadyReplicas == 0 && !stsUpdated) {
		err := addDeletionJob(svc, comp)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot create RBAC for the deletion job: %w", err))
		}
	}

	return nil
}

func needReleasePatch(comp *vshnv1.VSHNRedis, values map[string]interface{}) (bool, *xfnproto.Result) {
	releaseSizeValue, found, err := unstructured.NestedString(values, "master", "persistence", "size")
	if !found {
		return false, runtime.NewFatalResult(fmt.Errorf("disk size not found in release"))
	}
	if err != nil {
		return false, runtime.NewFatalResult(fmt.Errorf("failed to read size from release: %w", err))
	}

	desiredInt, err := getSizeAsInt(comp.Spec.Parameters.Size.Disk)
	if err != nil {
		return false, runtime.NewFatalResult(fmt.Errorf("cannot parse desired disk size: %w", err))
	}

	releaseInt, err := getSizeAsInt(releaseSizeValue)
	if err != nil {
		return false, runtime.NewFatalResult(fmt.Errorf("cannot parse release disk size: %w", err))
	}

	return desiredInt > releaseInt, nil

}

func getSizeAsInt(size string) (int64, error) {
	parsedSize, err := resource.ParseQuantity(size)
	if err != nil {
		return 0, err
	}
	finalSize, _ := parsedSize.AsInt64()
	return finalSize, nil
}

func getObservedOrDesiredRelease(svc *runtime.ServiceRuntime) (*helmv1beta1.Release, error) {
	r := &helmv1beta1.Release{}
	err := svc.GetObservedComposedResource(r, redisRelease)
	if err != nil && err != runtime.ErrNotFound {
		return nil, fmt.Errorf("cannot get redis release from observed iof: %v", err)
	}

	if err == runtime.ErrNotFound {
		err := svc.GetObservedComposedResource(r, redisRelease)
		if err != nil {
			return nil, fmt.Errorf("cannot get redis release from desired iof: %v", err)
		}
	}
	return r, nil
}

func addDeletionJob(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNRedis) error {
	ns := svc.Config.Data["controlNamespace"]

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.Name + "-sts-deletion-job",
			Namespace: ns,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "sa-sts-deleter",
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "sts-deleter",
							Image:           "bitnami/kubectl",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name:  "STS_NAME",
									Value: "redis-master",
								},
								{
									Name:  "STS_NAMESPACE",
									Value: getInstanceNamespace(comp),
								},
								{
									Name:  "STS_SIZE",
									Value: comp.Spec.Parameters.Size.Disk,
								},
								{
									Name:  "COMPOSITION_NAME",
									Value: comp.Name,
								},
							},
							Command: []string{
								"bash",
								"-c",
							},
							Args: []string{
								stsRecreateScript,
							},
						},
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObject(job, comp.Name+"-sts-deleter")
}

func addStsObserver(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNRedis) error {

	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-master",
			Namespace: getInstanceNamespace(comp),
		},
	}

	return svc.SetDesiredKubeObject(statefulset, comp.Name+"-sts-observer", runtime.KubeOptionObserve)
}
