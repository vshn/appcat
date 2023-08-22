package vshnredis

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	helmv1beta1 "github.com/crossplane-contrib/provider-helm/apis/release/v1beta1"
	xkubev1 "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
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
func ResizePVCs(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	comp := &vshnv1.VSHNRedis{}
	err := iof.Desired.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "failed to parse composite", err)
	}

	if comp.Spec.Parameters.Size.Disk == "" {
		return runtime.NewNormal()
	}

	release, err := getObservedOrDesiredRelease(ctx, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot get release", err)
	}

	values, err := getReleaseValues(release)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot parse release values", err)
	}

	err = addStsObserver(ctx, iof, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot observe sts", err)
	}

	patch, result := needReleasePatch(ctx, comp, values)
	if result != nil {
		return result
	}

	if patch {
		err = addDeletionJob(ctx, iof, comp)
		if err != nil {
			return runtime.NewFatalErr(ctx, "cannot create RBAC for the deletion job", err)
		}

		return runtime.NewNormal()
	}

	xJob := &xkubev1.Object{}
	err = iof.Observed.Get(ctx, xJob, comp.Name+"-sts-deleter")
	if err != nil && err != runtime.ErrNotFound {
		return runtime.NewFatalErr(ctx, "cannot get observed deletion job", err)
	}
	// If there's no job observed, we're done here.
	if err == runtime.ErrNotFound {
		return runtime.NewNormal()
	}

	sts := &appsv1.StatefulSet{}
	err = iof.Observed.GetFromObject(ctx, sts, comp.Name+"-sts-observer")
	if err != nil && err != runtime.ErrNotFound {
		return runtime.NewFatalErr(ctx, "cannot get observed statefulset job", err)
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
		return runtime.NewFatalErr(ctx, "cannot parse desired size", err)
	}
	stsUpdated := stsSize == desiredSize

	deletionJob := &batchv1.Job{}
	if observedJob {
		err := json.Unmarshal(xJob.Status.AtProvider.Manifest.Raw, deletionJob)
		if err != nil {
			return runtime.NewFatalErr(ctx, "cannot unmarshal sts deleter job", err)
		}
	}

	// The job hasn't been observed yet, so we need to keep it in desired, or we will have a recreate loop
	// Also as long as it hasn't finished we need to make sure it exists.
	if (!observedJob || deletionJob.Status.Succeeded < 1) || (sts.Status.ReadyReplicas == 0 && !stsUpdated) {
		err := addDeletionJob(ctx, iof, comp)
		if err != nil {
			return runtime.NewFatalErr(ctx, "cannot create RBAC for the deletion job", err)
		}
	}

	return runtime.NewNormal()
}

func needReleasePatch(ctx context.Context, comp *vshnv1.VSHNRedis, values map[string]interface{}) (bool, runtime.Result) {
	releaseSizeValue, found, err := unstructured.NestedString(values, "master", "persistence", "size")
	if !found {
		return false, runtime.NewFatalErr(ctx, "could not find disk size in release", fmt.Errorf("disk size not found in release"))
	}
	if err != nil {
		return false, runtime.NewFatalErr(ctx, "failed to read size from release", err)
	}

	desiredInt, err := getSizeAsInt(comp.Spec.Parameters.Size.Disk)
	if err != nil {
		return false, runtime.NewFatalErr(ctx, "cannot parse desired disk size", err)
	}

	releaseInt, err := getSizeAsInt(releaseSizeValue)
	if err != nil {
		return false, runtime.NewFatalErr(ctx, "cannot parse release disk size", err)
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

func getObservedOrDesiredRelease(ctx context.Context, iof *runtime.Runtime) (*helmv1beta1.Release, error) {
	r := &helmv1beta1.Release{}
	err := iof.Observed.Get(ctx, r, redisRelease)
	if err != nil && err != runtime.ErrNotFound {
		return nil, fmt.Errorf("cannot get redis release from observed iof: %v", err)
	}

	if err == runtime.ErrNotFound {
		err := iof.Desired.Get(ctx, r, redisRelease)
		if err != nil {
			return nil, fmt.Errorf("cannot get redis release from desired iof: %v", err)
		}
	}
	return r, nil
}

func addDeletionJob(ctx context.Context, iof *runtime.Runtime, comp *vshnv1.VSHNRedis) error {
	ns := iof.Config.Data["controlNamespace"]

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

	return iof.Desired.PutIntoObject(ctx, job, comp.Name+"-sts-deleter")
}

func addStsObserver(ctx context.Context, iof *runtime.Runtime, comp *vshnv1.VSHNRedis) error {

	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-master",
			Namespace: getInstanceNamespace(comp),
		},
	}

	return iof.Desired.PutIntoObserveOnlyObject(ctx, statefulset, comp.Name+"-sts-observer")
}
