package vshnforgejo

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
func AddMaintenanceJob(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	common.SetRandomSchedules(comp, comp)

	instanceNamespace := comp.GetInstanceNamespace()
	schedule := comp.GetFullMaintenanceSchedule()

	username := svc.Config.Data["registry_username"]
	password := svc.Config.Data["registry_password"]

	if err := svc.SetDesiredCompositeStatus(comp); err != nil {
		svc.Log.Error(err, "cannot set schedules in the composite status")
	}

	return maintenance.New(comp, svc, schedule, instanceNamespace, comp.GetServiceName()).
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
