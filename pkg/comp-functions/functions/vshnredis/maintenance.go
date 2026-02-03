package vshnredis

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
func AddMaintenanceJob(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) *xfnproto.Result {
	// Get the desired composite first to preserve CurrentReleaseTag if pinImageTag is set
	desiredComp := &vshnv1.VSHNRedis{}
	_ = svc.GetDesiredComposite(desiredComp)

	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	// Preserve CurrentReleaseTag from desired state if pinImageTag is set
	if comp.Spec.Parameters.Maintenance.PinImageTag != "" && desiredComp.Status.CurrentReleaseTag != "" {
		comp.Status.CurrentReleaseTag = desiredComp.Status.CurrentReleaseTag
	}

	maintTime := common.SetRandomMaintenanceSchedule(comp)
	common.SetRandomBackupSchedule(comp, &maintTime)

	instanceNamespace := getInstanceNamespace(comp)
	schedule := comp.GetFullMaintenanceSchedule()

	if err := svc.SetDesiredCompositeStatus(comp); err != nil {
		svc.Log.Error(err, "cannot set schedules in the composite status")
	}

	return maintenance.New(comp, svc, schedule, instanceNamespace, comp.GetServiceName()).
		WithHelmBasedService().
		Run(ctx)
}
