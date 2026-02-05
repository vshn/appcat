package vshnminio

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// AddMaintenanceJob will add a job to do the maintenance for the instance
func AddMaintenanceJob(ctx context.Context, comp *vshnv1.VSHNMinio, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetDesiredComposite(comp); err != nil {
		if err := svc.GetObservedComposite(comp); err != nil {
			return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
		}
	}

	maintTime := common.SetRandomMaintenanceSchedule(comp)
	common.SetRandomBackupSchedule(comp, &maintTime)

	if err := svc.SetDesiredCompositeStatus(comp); err != nil {
		svc.Log.Error(err, "cannot set schedules in the composite status")
	}

	instanceNamespace := getInstanceNamespace(comp)
	schedule := comp.GetFullMaintenanceSchedule()

	return maintenance.New(comp, svc, schedule, instanceNamespace, comp.GetServiceName()).
		WithHelmBasedService().
		Run(ctx)
}
