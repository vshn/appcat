package vshnredis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

	r, err := getRelease(ctx, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot get redis release from iof", err)
	}

	l.Info("Getting helm values")
	values, err := getReleaseValues(r)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot get values from redis release for composite "+comp.Name, err)
	}

	currentVersion, err := getCurrentReleaseVersion(ctx, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot get current redis version", err)
	}

	l.Info("Updating helm values")
	err = updateRelease(ctx, comp, r.Name, values, currentVersion)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot update redis release "+r.Name, err)
	}

	l.V(1).Info("Final value map", "map", values)

	byteValues, err := json.Marshal(values)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot marshal helm values for release "+r.Name, err)
	}

	r.Spec.ForProvider.Values.Raw = byteValues
	err = iof.Desired.PutWithResourceName(ctx, r, redisRelease)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot update release "+r.Name, err)
	}

	return runtime.NewNormal()
}

// During the very first reconcile the release is missing from the observed state therefore the release is taken one
// time from Desired. Subsequently, the release has to be taken from observed state as to make sure that updates
// from other jobs, such as maintenance, are reflected in the release.
func getRelease(ctx context.Context, iof *runtime.Runtime) (*v1beta1.Release, error) {
	r := &v1beta1.Release{}
	err := iof.Desired.Get(ctx, r, redisRelease)
	if err != nil {
		return nil, fmt.Errorf("cannot get redis release from desired iof: %v", err)
	}
	return r, nil
}

func updateRelease(ctx context.Context, comp *vshnv1.VSHNRedis, releaseName string, values map[string]interface{}, currentVersion string) error {
	l := controllerruntime.LoggerFrom(ctx)

	l.V(1).Info("Setting release version")
	err := setReleaseVersion(comp, values, currentVersion)
	if err != nil {
		return fmt.Errorf("cannot set redis version for release %s: %v", releaseName, err)
	}

	l.V(1).Info("Adding PVC annotations")
	err = addPVCAnnotation(values)
	if err != nil {
		return fmt.Errorf("cannot add pvc annotations for release %s: %v", releaseName, err)
	}

	l.V(1).Info("Adding pod annotations")
	err = addPodAnnotation(values)
	if err != nil {
		return fmt.Errorf("cannot add pod annotations for release %s: %v", releaseName, err)
	}

	l.V(1).Info("Adding backup config map")
	err = addBackupCM(values)
	if err != nil {
		return fmt.Errorf("cannot add configmap for release %s: %v", releaseName, err)
	}
	return nil
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
	err := json.Unmarshal(r.Spec.ForProvider.Values.Raw, &values)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal values from release: %v", err)
	}
	return values, nil
}

// setReleaseVersion sets the version from the claim if it's a new instance otherwise it is managed by maintenance function
func setReleaseVersion(comp *vshnv1.VSHNRedis, values map[string]interface{}, tag string) error {
	var err error
	if tag != "" {
		// In case the tag is set, keep the desired
		err = unstructured.SetNestedField(values, tag, "image", "tag")
	} else {
		// In case the tag is not set then this is a new Release therefore we need to set the version from the claim
		err = unstructured.SetNestedField(values, comp.Spec.Parameters.Service.Version, "image", "tag")
	}
	return err
}

func getCurrentReleaseVersion(ctx context.Context, iof *runtime.Runtime) (string, error) {
	r := &v1beta1.Release{}
	err := iof.Observed.Get(ctx, r, redisRelease)
	if errors.Is(err, runtime.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("cannot get redis release from observed iof: %v", err)
	}

	values, err := getReleaseValues(r)
	if err != nil {
		return "", fmt.Errorf("cannot get redis release values from observed iof: %v", err)
	}

	tag, _, err := unstructured.NestedString(values, "image", "tag")
	if err != nil {
		return "", fmt.Errorf("cannot get image tag from values in release: %v", err)
	}

	return tag, nil
}
