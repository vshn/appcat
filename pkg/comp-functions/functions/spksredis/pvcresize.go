package spksredis

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	helmv1beta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	xkubev1 "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	spksv1alpha1 "github.com/vshn/appcat/v4/apis/syntools/v1alpha1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var redisRelease = "redis-chart"

//go:embed script/recreate.sh
var stsRecreateScript string

// ResizeSpksPVCs will add a job to do the pvc resize for the instance
func ResizeSpksPVCs(ctx context.Context, comp *spksv1alpha1.CompositeRedisInstance, svc *runtime.ServiceRuntime) *xfnproto.Result {

	replicaKey := svc.Config.Data["replicaKey"]
	if replicaKey == "" {
		return runtime.NewFatalResult(fmt.Errorf("failed to parse replicaKey config value"))
	}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("failed to parse composite: %w", err))
	}

	release := &helmv1beta1.Release{}
	err = svc.GetDesiredComposedResourceByName(release, redisRelease)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get redis release from desired iof: %v", err))
	}

	values, err := common.GetReleaseValues(release)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot parse release values from desired release: %w", err))
	}

	if val, ok := release.GetAnnotations()["crossplane.io/paused"]; ok && val == "true" {
		// The release has just been updated and paused and is waiting for the deletion job to finish
		// The deletion job should remove the annotation once it's done.

		xJob := &xkubev1.Object{}
		err = svc.GetObservedComposedResource(xJob, comp.Name+"-sts-deleter")
		if err != nil && err != runtime.ErrNotFound {
			return runtime.NewFatalResult(fmt.Errorf("cannot get observed deletion job: %w", err))
		}
		// If there's no job observed, we're done here.
		if err == runtime.ErrNotFound {
			return runtime.NewFatalResult(fmt.Errorf("cannot get observed deletion job, but release is paused: %w", err))
		}

		sts := &appsv1.StatefulSet{}
		err = svc.GetObservedKubeObject(sts, comp.GetName()+"-sts-observer")
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
		newSize, found, err := unstructured.NestedString(values, replicaKey, "persistence", "size")
		if !found {
			return runtime.NewFatalResult(fmt.Errorf("disk size not found in observed release"))
		}

		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("failed to read size from observed release: %w", err))
		}
		desiredSize, err := getSizeAsInt(newSize)
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
			err := addDeletionJob(svc, comp, newSize, release.GetName(), replicaKey)
			if err != nil {
				return runtime.NewFatalResult(fmt.Errorf("cannot create RBAC for the deletion job: %w", err))
			}
		}
		return nil
	}

	err = addStsObserver(svc, comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot observe sts: %w", err).Error())
	}

	sts := &appsv1.StatefulSet{}
	err = svc.GetObservedKubeObject(sts, comp.GetName()+"-sts-observer")
	if err == runtime.ErrNotFound {
		return nil
	}
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed statefulset: %w", err))
	}

	stsSize := int64(0)
	// Check the current size in the sts
	if len(sts.Spec.VolumeClaimTemplates) > 0 {
		stsSize, _ = sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().AsInt64()
	}

	patch, newSize, result := needReleasePatch(values, stsSize, replicaKey)
	if result != nil {
		return result
	}

	if patch {
		err = addDeletionJob(svc, comp, newSize, release.GetName(), replicaKey)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot create the deletion job: %w", err))
		}
		// We pause the release at this point to make sure that provider-helm doesn't update the
		// release until the deletion job removed the sts
		release.SetAnnotations(map[string]string{
			"crossplane.io/paused": "true",
		})
		err = svc.SetDesiredComposedResourceWithName(release, redisRelease)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("Can't pause the release: %w", err))
		}
		return nil
	}

	return nil
}

func addStsObserver(svc *runtime.ServiceRuntime, comp *spksv1alpha1.CompositeRedisInstance) error {
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-node",
			Namespace: comp.GetName(),
		},
	}

	providerConfigRef := comp.GetLabels()["service.syn.tools/cluster"]

	if providerConfigRef == "" {
		return fmt.Errorf("Can't get cluster from label")
	}

	return svc.SetDesiredKubeObject(statefulset, comp.GetName()+"-sts-observer", KubeOptionAddProviderRef(providerConfigRef, true))
}

func needReleasePatch(values map[string]interface{}, stsSize int64, replicaKey string) (bool, string, *xfnproto.Result) {
	releaseSizeValue, found, err := unstructured.NestedString(values, replicaKey, "persistence", "size")
	if !found {
		return false, "", runtime.NewFatalResult(fmt.Errorf("disk size not found in observed release"))
	}

	if err != nil {
		return false, "", runtime.NewFatalResult(fmt.Errorf("failed to read size from observed release: %w", err))
	}

	releaseInt, err := getSizeAsInt(releaseSizeValue)
	if err != nil {
		return false, "", runtime.NewFatalResult(fmt.Errorf("cannot parse release disk size from observed releaes: %w", err))
	}

	return releaseInt > stsSize, releaseSizeValue, nil

}

func getSizeAsInt(size string) (int64, error) {
	parsedSize, err := resource.ParseQuantity(size)
	if err != nil {
		return 0, err
	}
	finalSize, _ := parsedSize.AsInt64()
	return finalSize, nil
}

func addDeletionJob(svc *runtime.ServiceRuntime, comp *spksv1alpha1.CompositeRedisInstance, newSize string, releaseName string, replicaKey string) error {
	ns := svc.Config.Data["spksNamespace"]
	stsDeleterImage := svc.Config.Data["stsDeleterImage"]

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.Name + "sts-deletion-job",
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
							Image:           stsDeleterImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name:  "STS_NAME",
									Value: "redis-node",
								},
								{
									Name:  "STS_NAMESPACE",
									Value: comp.GetName(),
								},
								{
									Name:  "STS_SIZE",
									Value: newSize,
								},
								{
									Name:  "RELEASE_NAME",
									Value: releaseName,
								},
								{
									Name:  "REPLICA_KEY",
									Value: replicaKey,
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

	providerConfigRef := comp.GetLabels()["service.syn.tools/cluster"]

	if providerConfigRef == "" {
		return fmt.Errorf("Can't get cluster from label")
	}

	return svc.SetDesiredKubeObject(job, comp.Name+"-sts-deleter", KubeOptionAddProviderRef(providerConfigRef, false))
}

// KubeOptionAddProviderRef adds the given correct ProviderConfigRef to the kube object.
func KubeOptionAddProviderRef(providerConfigName string, observer bool) runtime.KubeObjectOption {
	return func(obj *xkubev1.Object) {
		obj.Spec.ProviderConfigReference.Name = providerConfigName
		if observer {
			obj.Spec.ManagementPolicies = nil
			obj.Spec.ManagementPolicies = append(obj.Spec.ManagementPolicies, xpv1.ManagementActionObserve)
		}
	}
}
