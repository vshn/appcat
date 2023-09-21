package vshnminio

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

var service = "minio"

// AddMaintenanceJob will add a job to do the maintenance for the instance
func AddMaintenanceJob(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	comp := &vshnv1.VSHNMinio{}
	err := iof.Observed.GetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "can't get composite", err)
	}

	common.SetRandomSchedules(&comp.Spec.Parameters.Backup, &comp.Spec.Parameters.Maintenance)

	err = iof.Desired.SetComposite(ctx, comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot update composite", err)
	}

	instanceNamespace := getInstanceNamespace(comp)
	schedule := comp.Spec.Parameters.Maintenance

	return maintenance.New(comp, iof, schedule, instanceNamespace, service).
		WithHelmBasedService().
		Run(ctx)
}
