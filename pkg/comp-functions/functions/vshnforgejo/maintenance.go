package vshnforgejo

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// AddMaintenanceJob will add a job to do the maintenance for the instance
func AddMaintenanceJob(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	common.SetRandomSchedules(comp, comp)

	instanceNamespace := comp.GetInstanceNamespace()
	schedule := comp.GetFullMaintenanceSchedule()

	err = svc.SetDesiredCompositeStatus(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot update composite status: %w", err))
	}

	return maintenance.New(comp, svc, schedule, instanceNamespace, comp.GetServiceName()).
		WithHelmBasedService().
		Run(ctx)
}
