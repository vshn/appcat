package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

// GetReleaseValues returns the parsed values from the given release.
func GetReleaseValues(r *xhelmv1.Release) (map[string]interface{}, error) {
	values := map[string]interface{}{}
	if r == nil {
		return values, nil
	}

	if r.Spec.ForProvider.Values.Raw == nil {
		return values, nil
	}

	err := json.Unmarshal(r.Spec.ForProvider.Values.Raw, &values)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal values from release: %v", err)
	}
	return values, nil
}

// GetObservedReleaseValues returns the observed releaseValues for the given release name.
func GetObservedReleaseValues(svc *runtime.ServiceRuntime, releaseName string) (map[string]interface{}, error) {
	r, err := getObservedRelease(svc, releaseName)
	if err != nil {
		return nil, fmt.Errorf("cannot get observed release: %w", err)
	}

	return GetReleaseValues(r)
}

// GetDesiredReleaseValues returns the desired releaseValues for the given release name.
func GetDesiredReleaseValues(svc *runtime.ServiceRuntime, releaseName string) (map[string]interface{}, error) {
	r, err := getDesiredRelease(svc, releaseName)
	if err != nil {
		return nil, fmt.Errorf("cannot get desired release: %w", err)
	}

	return GetReleaseValues(r)
}

func getObservedRelease(svc *runtime.ServiceRuntime, releaseName string) (*xhelmv1.Release, error) {
	r := &xhelmv1.Release{}
	err := svc.GetObservedComposedResource(r, releaseName)
	if errors.Is(err, runtime.ErrNotFound) {
		return nil, nil
	}
	return r, nil
}

func getDesiredRelease(svc *runtime.ServiceRuntime, releaseName string) (*xhelmv1.Release, error) {
	r := &xhelmv1.Release{}
	err := svc.GetDesiredComposedResourceByName(r, releaseName)
	if errors.Is(err, runtime.ErrNotFound) {
		return nil, nil
	}
	return r, nil
}

// NewRelease returns a new release with some defaults set.
func NewRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp InfoGetter, values map[string]any, cd ...xhelmv1.ConnectionDetail) (*xhelmv1.Release, error) {

	vb, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	release := &xhelmv1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName(),
		},
		Spec: xhelmv1.ReleaseSpec{
			ForProvider: xhelmv1.ReleaseParameters{
				Chart: xhelmv1.ChartSpec{
					Repository: svc.Config.Data["chartRepository"],
					Version:    svc.Config.Data["chartVersion"],
					Name:       comp.GetServiceName(),
				},
				Namespace: comp.GetInstanceNamespace(),
				ValuesSpec: xhelmv1.ValuesSpec{
					Values: k8sruntime.RawExtension{
						Raw: vb,
					},
				},
			},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: "helm",
				},
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      comp.GetName() + "-connection",
					Namespace: comp.GetInstanceNamespace(),
				},
			},
			ConnectionDetails: cd,
		},
	}

	if strings.HasPrefix(svc.Config.Data["chartRepository"], "oci://") {
		release.Spec.ForProvider.Chart.URL = fmt.Sprintf("%s:%s", svc.Config.Data["chartRepository"], svc.Config.Data["chartVersion"])
	}

	return release, nil
}
