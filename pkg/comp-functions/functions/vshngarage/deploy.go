package vshngarage

import (
	"context"
	"errors"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
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

	err = common.CustomCreateNetworkPolicy([]string{"syn-garage-operator"}, comp.GetInstanceNamespace(), "allow-garage-operator", "allow-garage-operator", false, svc)
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
		"storageDataSpace":     calcResources.Disk.String(),
		"storageMetadataSpace": comp.Spec.Parameters.Service.MetadataStorage,
	}

	connectionDetails := []v1beta1.ConnectionDetail{
		{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "garage.rajsingh.info/v1alpha1",
				Kind:       "GarageCluster",
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
