package vshnredis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var redisRelease = "release"

// ManageRelease will update the release in line with other composition functions
func ManageRelease(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	l := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNRedis{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	desiredRelease, err := getRelease(ctx, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get redis release from iof: %w", err))
	}
	observedRelease, err := getObservedRelease(ctx, svc)
	if err != nil {
		return runtime.NewWarningResult("cannot get observed release, skipping")
	}

	releaseName := desiredRelease.Name

	l.Info("Updating helm values")
	desiredRelease, err = updateRelease(ctx, comp, desiredRelease, observedRelease)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot update redis release %q: %q: %w", releaseName, err.Error(), err))
	}

	err = svc.SetDesiredComposedResourceWithName(desiredRelease, redisRelease)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot update release %q: %w", releaseName, err))
	}

	return nil
}

func getRelease(ctx context.Context, svc *runtime.ServiceRuntime) (*v1beta1.Release, error) {
	r := &v1beta1.Release{}
	err := svc.GetDesiredComposedResourceByName(r, redisRelease)
	if err != nil {
		return nil, fmt.Errorf("cannot get redis release from desired iof: %v", err)
	}
	return r, nil
}

func getObservedRelease(ctx context.Context, svc *runtime.ServiceRuntime) (*v1beta1.Release, error) {
	r := &v1beta1.Release{}
	err := svc.GetObservedComposedResource(r, redisRelease)
	if errors.Is(err, runtime.ErrNotFound) {
		return r, nil
	}
	return r, err
}

func updateRelease(ctx context.Context, comp *vshnv1.VSHNRedis, desired *v1beta1.Release, observed *v1beta1.Release) (*v1beta1.Release, error) {
	l := controllerruntime.LoggerFrom(ctx)
	l.Info("Getting helm values")
	releaseName := desired.Name

	values, err := getReleaseValues(desired)
	if err != nil {
		return nil, err
	}
	observedValues, err := getReleaseValues(observed)
	if err != nil {
		return nil, err
	}

	l.V(1).Info("Setting release version")
	err = maintenance.SetReleaseVersion(ctx, comp.Spec.Parameters.Service.Version, values, observedValues, []string{"image", "tag"})
	if err != nil {
		return nil, fmt.Errorf("cannot set redis version for release %s: %v", releaseName, err)
	}

	l.V(1).Info("Adding PVC annotations")
	err = addPVCAnnotation(values)
	if err != nil {
		return nil, fmt.Errorf("cannot add pvc annotations for release %s: %v", releaseName, err)
	}

	l.V(1).Info("Adding pod annotations")
	err = addPodAnnotation(values)
	if err != nil {
		return nil, fmt.Errorf("cannot add pod annotations for release %s: %v", releaseName, err)
	}

	l.V(1).Info("Adding backup config map")
	err = addBackupCM(values)
	if err != nil {
		return nil, fmt.Errorf("cannot add configmap for release %s: %v", releaseName, err)
	}

	l.V(1).Info("Final value map", "map", values)

	byteValues, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	desired.Spec.ForProvider.Values.Raw = byteValues

	return desired, nil
}

func addPVCAnnotation(valueMap map[string]any) error {
	annotations := map[string]interface{}{
		"k8up.io/backup": "false",
	}
	err := unstructured.SetNestedMap(valueMap, annotations, "master", "persistence", "annotations")
	if err != nil {
		return fmt.Errorf("cannot set annotations the helm values for key: master.persistence")
	}

	return nil
}

func addPodAnnotation(valueMap map[string]any) error {
	annotations := map[string]interface{}{
		"k8up.io/backupcommand":  "/scripts/backup.sh",
		"k8up.io/file-extension": ".tar",
	}
	err := unstructured.SetNestedMap(valueMap, annotations, "master", "podAnnotations")
	if err != nil {
		return fmt.Errorf("cannot set annotations the helm values for key: master.podAnnotations")
	}

	return nil
}

func addBackupCM(valueMap map[string]any) error {
	masterMap, ok := valueMap["master"].(map[string]any)
	if !ok {
		return fmt.Errorf("cannot parse the helm values for key: master")
	}

	volumes := []corev1.Volume{
		{
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

	masterMap["extraVolumes"] = volumes

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      backupScriptCMName,
			MountPath: "/scripts",
		},
	}

	masterMap["extraVolumeMounts"] = volumeMounts

	valueMap["master"] = masterMap

	return nil
}

func getReleaseValues(r *v1beta1.Release) (map[string]interface{}, error) {
	values := map[string]interface{}{}
	if r.Spec.ForProvider.Values.Raw == nil {
		return values, nil
	}
	err := json.Unmarshal(r.Spec.ForProvider.Values.Raw, &values)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal values from release: %v", err)
	}
	return values, nil
}
