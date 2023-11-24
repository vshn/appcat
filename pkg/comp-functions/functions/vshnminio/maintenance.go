package vshnminio

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

var service = "minio"

// AddMaintenanceJob will add a job to do the maintenance for the instance
func AddMaintenanceJob(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	comp := &vshnv1.VSHNMinio{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		err = fmt.Errorf("cannot get observed composite: %w", err)
		return runtime.NewFatalResult(err)
	}

	common.SetRandomSchedules(comp, comp)

	err = svc.SetDesiredCompositeStatus(comp)
	if err != nil {
		err = fmt.Errorf("cannot set desired composite status: %w", err)
		return runtime.NewFatalResult(err)
	}

	instanceNamespace := getInstanceNamespace(comp)
	schedule := comp.GetFullMaintenanceSchedule()

	return maintenance.New(comp, svc, schedule, instanceNamespace, service).
		WithHelmBasedService().
		Run(ctx)
}
