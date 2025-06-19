package vshnkeycloak

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

// AddMaintenanceJob will add a job to do the maintenance for the instance
func AddMaintenanceJob(ctx context.Context, comp *vshnv1.VSHNKeycloak, svc *runtime.ServiceRuntime) *xfnproto.Result {
	observedComp := &vshnv1.VSHNKeycloak{}
	if err := svc.GetObservedComposite(observedComp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get observed composite: %w", err))
	}

	desiredComp := comp
	if err := svc.GetDesiredComposite(desiredComp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get desired composite: %w", err))
	}

	common.SetRandomSchedules(desiredComp, desiredComp)

	instanceNamespace := desiredComp.GetInstanceNamespace()
	schedule := desiredComp.GetFullMaintenanceSchedule()

	username := svc.Config.Data["registry_username"]
	password := svc.Config.Data["registry_password"]

	if err := svc.SetDesiredCompositeStatus(desiredComp); err != nil {
		svc.Log.Error(err, "cannot set schedules in the composite status")
	}

	return maintenance.New(desiredComp, svc, schedule, instanceNamespace, desiredComp.GetServiceName()).
		WithHelmBasedService().
		WithExtraEnvs([]corev1.EnvVar{
			{
				Name:  "REGISTRY_USERNAME",
				Value: username,
			},
			{
				Name:  "REGISTRY_PASSWORD",
				Value: password,
			},
		}...).
		Run(ctx)
}
