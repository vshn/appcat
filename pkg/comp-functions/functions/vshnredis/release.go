package vshnredis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/crossplane-contrib/provider-helm/apis/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var redisRelease = "release"

// ManageRelease will update the release in line with other composition functions
func ManageRelease(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	l := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNRedis{}
	err := iof.Observed.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "can't get composite", err)
	}

	desiredRelease, err := getRelease(ctx, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot get redis release from iof", err)
	}
	observedRelease, err := getObservedRelease(ctx, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot get observed release", err)
	}

	releaseName := desiredRelease.Name

	l.Info("Updating helm values")
	desiredRelease, err = updateRelease(ctx, comp, desiredRelease, observedRelease)
	if err != nil {
		return runtime.NewFatalErr(ctx, fmt.Sprintf("cannot update redis release %q: %q", releaseName, err.Error()), err)
	}

	err = iof.Desired.PutWithResourceName(ctx, desiredRelease, redisRelease)
	if err != nil {
		return runtime.NewFatalErr(ctx, fmt.Sprintf("cannot update release %q", releaseName), err)
	}

	return runtime.NewNormal()
}

func getRelease(ctx context.Context, iof *runtime.Runtime) (*v1beta1.Release, error) {
	r := &v1beta1.Release{}
	err := iof.Desired.Get(ctx, r, redisRelease)
	if err != nil {
		return nil, fmt.Errorf("cannot get redis release from desired iof: %v", err)
	}
	return r, nil
}

func getObservedRelease(ctx context.Context, iof *runtime.Runtime) (*v1beta1.Release, error) {
	r := &v1beta1.Release{}
	err := iof.Observed.Get(ctx, r, redisRelease)
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
	err = setReleaseVersion(ctx, comp, values, observedValues)
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

// setReleaseVersion sets the version from the claim if it's a new instance otherwise it is managed by maintenance function
func setReleaseVersion(ctx context.Context, comp *vshnv1.VSHNRedis, values map[string]interface{}, observed map[string]interface{}) error {
	l := controllerruntime.LoggerFrom(ctx)

	tag, _, err := unstructured.NestedString(observed, "image", "tag")
	if err != nil {
		return fmt.Errorf("cannot get image tag from values in release: %v", err)
	}

	desiredVersion, err := semver.ParseTolerant(comp.Spec.Parameters.Service.Version)
	if err != nil {
		l.Info("failed to parse desired redis version", "version", comp.Spec.Parameters.Service.Version)
		return fmt.Errorf("invalid redis version %q", comp.Spec.Parameters.Service.Version)
	}

	observedVersion, err := semver.ParseTolerant(tag)
	if err != nil {
		l.Info("failed to parse observed redis version", "version", tag)
		// If the observed version is not parsable, e.g. if it's empty, update to the desired version
		return unstructured.SetNestedField(values, comp.Spec.Parameters.Service.Version, "image", "tag")
	}

	if observedVersion.GTE(desiredVersion) {
		// In case the overved tag is valid and greater than the desired version, keep the observed version
		return unstructured.SetNestedField(values, tag, "image", "tag")
	}
	// In case the observed tag is smaller than the desired version,  then set the version from the claim
	return unstructured.SetNestedField(values, comp.Spec.Parameters.Service.Version, "image", "tag")
}
