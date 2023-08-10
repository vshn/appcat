package vshnpostgres

import (
	"context"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

// TransformSchedule initializes the backup and maintenance schedules  if the user did not explicitly provide a schedule.
func TransformSchedule(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	comp := vshnv1.VSHNPostgreSQL{}
	err := iof.Desired.GetComposite(ctx, &comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "failed to parse composite", err)
	}

	common.SetRandomSchedules(&comp.Spec.Parameters.Backup, &comp.Spec.Parameters.Maintenance)

	err = iof.Desired.SetComposite(ctx, &comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "failed to set composite", err)
	}

	return runtime.NewNormal()
}
