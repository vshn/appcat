package vshngarage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/vshn/appcat/v4/apis/garage"
	"github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

const (
	// GarageHost is env variable in the connection secret
	GarageHost = "GARAGE_URL"
)

// applyAllowedNamespaces decodes the comma-coupled JSON list provided through
// the comp-function xfn-config (key: garageAllowedNamespaces) and injects it
// into the vshngaragecluster chart values. Empty input leaves values
// untouched so older component-appcat releases that don't ship the key keep
// working — the chart simply skips the GarageReferenceGrant template.
func applyAllowedNamespaces(values map[string]any, raw string) error {
	if raw == "" {
		return nil
	}
	var allowed []string
	if err := json.Unmarshal([]byte(raw), &allowed); err != nil {
		return fmt.Errorf("cannot parse garageAllowedNamespaces %q: %w", raw, err)
	}
	if len(allowed) > 0 {
		values["allowedNamespaces"] = allowed
	}
	return nil
}

func DeployGarage(ctx context.Context, comp *vshnv1.VSHNGarage, svc *runtime.ServiceRuntime) *xfnproto.Result {
	l := svc.Log

	l.Info("Deploying Garage...")

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed composite: %w", err))
	}

	l.Info("Bootstrapping instance namespace and rbac rules")
	err = common.BootstrapInstanceNs(ctx, comp, "garage", "instanceNamespace", svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot bootstrap instance namespace: %w", err).Error())
	}

	err = common.CustomCreateNetworkPolicy([]string{"syn-garage-operator"}, comp.GetInstanceNamespace(), "allow-garage-operator", comp.GetName()+"-allow-garage-operator", false, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create network policies for garage operator: %w", err))
	}

	resources, err := utils.FetchPlansFromConfig(ctx, svc, comp.GetSize().Plan)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get plans: %w", err))
	}

	calcResources, errs := common.GetResources(&comp.Spec.Parameters.Size, resources)
	if len(errs) > 0 {
		return runtime.NewFatalResult(fmt.Errorf("cannot calculate resources: %w", errors.Join(errs...)))
	}

	values := map[string]any{
		"replicaCount": comp.GetInstances(),
		"isOpenshift":  svc.Config.Data["isOpenshift"] == "true",
		"resources": map[string]any{
			"requests": map[string]any{
				"cpu":    calcResources.ReqCPU,
				"memory": calcResources.ReqMem,
			},
			"limits": map[string]any{
				"cpu":    calcResources.CPU,
				"memory": calcResources.Mem,
			},
		},
		"adminKey": map[string]any{
			"enabled": true,
		},
		"storageDataSpace":     calcResources.Disk.String(),
		"storageMetadataSpace": comp.Spec.Parameters.Service.MetadataStorage,
	}

	if err := applyAllowedNamespaces(values, svc.Config.Data["garageAllowedNamespaces"]); err != nil {
		return runtime.NewFatalResult(err)
	}

	connectionDetails := []v1beta1.ConnectionDetail{
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: garage.GroupVersion.String(),
				Kind:       garage.GarageClusterGVK.Kind,
				Name:       "garage",
				Namespace:  comp.GetInstanceNamespace(),
				FieldPath:  "status.endpoints.s3",
			},
			ToConnectionSecretKey: GarageHost,
		},
	}

	release, err := common.NewRelease(ctx, svc, comp, values, comp.GetName()+"-garage", connectionDetails...)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create release: %w", err))
	}

	release.Spec.ForProvider.Chart.Name = "vshngaragecluster"

	err = svc.SetDesiredComposedResource(release)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set desired release: %w", err))
	}

	err = svc.AddObservedConnectionDetails(comp.GetName())
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add observed connection details for garage: %s", err))
	}

	return nil
}
