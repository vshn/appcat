package vshnredis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/backup"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var redisRelease = "release"

// ManageRelease will update the release in line with other composition functions
func ManageRelease(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) *xfnproto.Result {

	l := controllerruntime.LoggerFrom(ctx)

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

	l.Info("Creating password secret")
	passwordSecret, err := common.AddCredentialsSecret(comp, svc, []string{"root-password"})
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create credential secret: %s", err))
	}

	l.Info("Updating helm values")
	desiredRelease, err = updateRelease(ctx, comp, desiredRelease, observedRelease, passwordSecret)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot update redis release %q: %q: %w", releaseName, err.Error(), err))
	}

	err = svc.AddObservedConnectionDetails("release")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add release's connection details: %s", err))
	}

	err = svc.SetDesiredComposedResourceWithName(desiredRelease, redisRelease)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot update release %q: %w", releaseName, err))
	}

	return nil
}

func getRelease(ctx context.Context, svc *runtime.ServiceRuntime) (*xhelmv1.Release, error) {
	r := &xhelmv1.Release{}
	err := svc.GetDesiredComposedResourceByName(r, redisRelease)
	if err != nil {
		return nil, fmt.Errorf("cannot get redis release from desired iof: %v", err)
	}
	return r, nil
}

func getObservedRelease(ctx context.Context, svc *runtime.ServiceRuntime) (*xhelmv1.Release, error) {
	r := &xhelmv1.Release{}
	err := svc.GetObservedComposedResource(r, redisRelease)
	if errors.Is(err, runtime.ErrNotFound) {
		return r, nil
	}
	return r, err
}

func updateRelease(ctx context.Context, comp *vshnv1.VSHNRedis, desired *xhelmv1.Release, observed *xhelmv1.Release, passwordSecret string) (*xhelmv1.Release, error) {
	l := controllerruntime.LoggerFrom(ctx)
	l.Info("Getting helm values")
	releaseName := desired.Name

	values, err := common.GetReleaseValues(desired)
	if err != nil {
		return nil, err
	}
	observedValues, err := common.GetReleaseValues(observed)
	if err != nil {
		return nil, err
	}

	l.V(1).Info("Setting release version")
	_, err = maintenance.SetReleaseVersion(ctx, comp.Spec.Parameters.Service.Version, values, observedValues, []string{"image", "tag"})
	if err != nil {
		return nil, fmt.Errorf("cannot set redis version for release %s: %v", releaseName, err)
	}

	l.V(1).Info("Adding PVC annotations")
	err = backup.AddPVCAnnotationToValues(values, "master", "persistence", "annotations")
	if err != nil {
		return nil, fmt.Errorf("cannot add pvc annotations for release %s: %v", releaseName, err)
	}

	l.V(1).Info("Adding pod annotations")
	err = backup.AddPodAnnotationToValues(values, "/scripts/backup.sh", ".tar", "master", "podAnnotations")
	if err != nil {
		return nil, fmt.Errorf("cannot add pod annotations for release %s: %v", releaseName, err)
	}

	l.V(1).Info("Adding backup config map")
	err = backup.AddBackupCMToValues(values, []string{"master", "extraVolumes"}, []string{"master", "extraVolumeMounts"})
	if err != nil {
		return nil, fmt.Errorf("cannot add configmap for release %s: %v", releaseName, err)
	}

	l.V(1).Info("Adding secret reference")
	err = addPassword(values, passwordSecret)
	if err != nil {
		return nil, fmt.Errorf("cannot add password secret for release %s: %w", releaseName, err)
	}

	l.V(1).Info("Add connection details to release")
	addConnectionDetails(desired, comp.GetInstanceNamespace(), passwordSecret)

	l.V(1).Info("Final value map", "map", values)

	byteValues, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	desired.Spec.ForProvider.Values.Raw = byteValues

	return desired, nil
}

func addPassword(valueMap map[string]interface{}, secretName string) error {
	err := unstructured.SetNestedField(valueMap, secretName, "auth", "existingSecret")
	if err != nil {
		return err
	}
	return unstructured.SetNestedField(valueMap, "root-password", "auth", "existingSecretPasswordKey")
}

func addConnectionDetails(release *xhelmv1.Release, namespace, secretName string) {
	cd := []xhelmv1.ConnectionDetail{
		{
			ObjectReference: corev1.ObjectReference{
				Kind:       "Secret",
				APIVersion: "v1",
				Namespace:  namespace,
				Name:       secretName,
				FieldPath:  "data.root-password",
			},
			ToConnectionSecretKey:  "REDIS_PASSWORD",
			SkipPartOfReleaseCheck: true,
		},
		{
			ObjectReference: corev1.ObjectReference{
				Kind:       "Secret",
				APIVersion: "v1",
				Namespace:  namespace,
				Name:       "tls-client-certificate",
				FieldPath:  "data[ca.crt]",
			},
			ToConnectionSecretKey:  "ca.crt",
			SkipPartOfReleaseCheck: true,
		},
		{
			ObjectReference: corev1.ObjectReference{
				Kind:       "Secret",
				APIVersion: "v1",
				Namespace:  namespace,
				Name:       "tls-client-certificate",
				FieldPath:  "data[tls.crt]",
			},
			ToConnectionSecretKey:  "tls.crt",
			SkipPartOfReleaseCheck: true,
		},
		{
			ObjectReference: corev1.ObjectReference{
				Kind:       "Secret",
				APIVersion: "v1",
				Namespace:  namespace,
				Name:       "tls-client-certificate",
				FieldPath:  "data[tls.key]",
			},
			ToConnectionSecretKey:  "tls.key",
			SkipPartOfReleaseCheck: true,
		},
	}

	release.Spec.ResourceSpec.WriteConnectionSecretToReference = &xpv1.SecretReference{
		Name:      "release-connection-details",
		Namespace: namespace,
	}
	release.Spec.ConnectionDetails = cd
}
