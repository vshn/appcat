package vshnredis

import (
	"context"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

var service = "redis"

// AddMaintenanceJob will add a job to do the maintenance for the instance
func AddMaintenanceJob(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	comp := &vshnv1.VSHNRedis{}
	err := iof.Observed.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "can't get composite", err)
	}

	instanceNamespace := getInstanceNamespace(comp)
	schedule := comp.Spec.Parameters.Maintenance

	return maintenance.New(comp, iof, schedule, instanceNamespace, service).
		WithHelmBasedService().
		Run(ctx)
}
