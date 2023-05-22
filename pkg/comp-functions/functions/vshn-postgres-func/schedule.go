package vshnpostgres

import (
	"context"
	"fmt"
	"github.com/vshn/appcat-apiserver/pkg/comp-functions/runtime"
	"math/rand"
	"time"

	vshnv1 "github.com/vshn/appcat-apiserver/apis/vshn/v1"
)

var (
	maitenanceWindowStart = time.Date(1970, 1, 1, 21, 0, 0, 0, time.UTC)
	maitenanceWindowEnd   = time.Date(1970, 1, 2, 5, 0, 0, 0, time.UTC)
	maitenanceWindowRange = maitenanceWindowEnd.Unix() - maitenanceWindowStart.Unix()
)

// TransformSchedule initializes the backup and maintenance schedules  if the user did not explicitly provide a schedule.
// The maintenance will be set to a random time on Tuesday night between 21:00 and 5:00, and the backup schedule will be set to once a day between 20:00 and 4:00.
// If neither maintenance nor backup is set, the function will make sure that there will be backup scheduled one hour before the maintenance.
func TransformSchedule(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	comp := vshnv1.VSHNPostgreSQL{}
	err := iof.Desired.GetComposite(ctx, &comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "failed to parse composite", err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	maintTime := time.Unix(maitenanceWindowStart.Unix()+rng.Int63n(maitenanceWindowRange), 0).In(time.UTC)
	backupTime := maintTime.Add(-1 * time.Hour).In(time.UTC)

	if comp.Spec.Parameters.Backup.Schedule == "" {
		comp.Spec.Parameters.Backup.Schedule = fmt.Sprintf("%d %d * * *", backupTime.Minute(), backupTime.Hour())
	}

	if comp.Spec.Parameters.Maintenance.TimeOfDay == "" {
		comp.Spec.Parameters.Maintenance.TimeOfDay = maintTime.Format(time.TimeOnly)
	}

	if comp.Spec.Parameters.Maintenance.DayOfWeek == "" {
		day := "tuesday"
		if maintTime.Day() > 1 {
			day = "wednesday"
		}
		comp.Spec.Parameters.Maintenance.DayOfWeek = day
	}

	err = iof.Desired.SetComposite(ctx, &comp)
	if err != nil {
		return runtime.NewFatalErr(ctx, "failed to set composite", err)
	}

	return runtime.NewNormal()
}
