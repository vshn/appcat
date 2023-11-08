package vshnpostgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	// fnv1aplha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
)

func TestTransformSchedule_SetRandomSchedule(t *testing.T) {

	for i := 0; i < 50; i++ {
		t.Run(fmt.Sprintf("Round %d", i), func(t *testing.T) {
			svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/base.yaml")

			res := TransformSchedule(context.TODO(), svc)
			assert.Nil(t, res)

			out := &vshnv1.VSHNPostgreSQL{}
			err := svc.GetDesiredComposite(out)
			assert.NoError(t, err)

			backupTime := parseAndValidateBackupSchedule(t, out)
			maintTime := parseAndValidateMaitenance(t, out)

			t.Logf("Backup Time: %q\n", backupTime.Format(time.RFC3339))
			t.Logf("Maintenance Time: %q\n", maintTime.Format(time.RFC3339))

			diff := maintTime.Truncate(time.Minute).Sub(backupTime)
			assert.Equal(t, time.Hour, diff)
		})
	}

}

func TestTransformSchedule_DontOverwriteBackup(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/base.yaml")

	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetDesiredComposite(comp)
	assert.NoError(t, err)
	comp.Spec.Parameters.Backup.Schedule = "3 2 * * *"
	err = svc.GetDesiredComposite(comp)
	assert.NoError(t, err)

	res := TransformSchedule(context.TODO(), svc)
	assert.Nil(t, res)

	err = svc.GetDesiredComposite(comp)
	assert.NoError(t, err)
	assert.Equal(t, "3 2 * * *", comp.Spec.Parameters.Backup.Schedule)

	_ = parseAndValidateMaitenance(t, comp)
}

func TestTransformSchedule_DontOverwriteMaintenance(t *testing.T) {
	svc := commontest.LoadRuntimeFromFile(t, "vshn-postgres/base.yaml")

	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetDesiredComposite(comp)
	assert.NoError(t, err)
	comp.Spec.Parameters.Maintenance.DayOfWeek = "thursday"
	comp.Spec.Parameters.Maintenance.TimeOfDay = "11:12:23"
	err = svc.SetDesiredCompositeStatus(comp)
	assert.NoError(t, err)

	res := TransformSchedule(context.TODO(), svc)
	assert.Nil(t, res)

	err = svc.GetDesiredComposite(comp)
	assert.NoError(t, err)
	assert.Equal(t, "thursday", comp.Spec.Parameters.Maintenance.DayOfWeek)
	assert.Equal(t, "11:12:23", comp.Spec.Parameters.Maintenance.TimeOfDay)

	_ = parseAndValidateBackupSchedule(t, comp)
}

func TestTransformSchedule_DontOverwriteBackupOrMaintenance(t *testing.T) {
	iof := commontest.LoadRuntimeFromFile(t, "vshn-postgres/base.yaml")

	comp := &vshnv1.VSHNPostgreSQL{}
	err := iof.GetDesiredComposite(comp)
	assert.NoError(t, err)
	comp.Spec.Parameters.Backup.Schedule = "3 2 * * *"
	comp.Spec.Parameters.Maintenance.DayOfWeek = "thursday"
	comp.Spec.Parameters.Maintenance.TimeOfDay = "11:12:23"
	err = iof.SetDesiredCompositeStatus(comp)
	assert.NoError(t, err)

	res := TransformSchedule(context.TODO(), iof)
	assert.Nil(t, res)

	err = iof.GetDesiredComposite(comp)
	assert.NoError(t, err)
	assert.Equal(t, "3 2 * * *", comp.Spec.Parameters.Backup.Schedule)
	assert.Equal(t, "thursday", comp.Spec.Parameters.Maintenance.DayOfWeek)
	assert.Equal(t, "11:12:23", comp.Spec.Parameters.Maintenance.TimeOfDay)
}

func parseAndValidateBackupSchedule(t *testing.T, comp *vshnv1.VSHNPostgreSQL) time.Time {
	var backupTime time.Time
	t.Run("validateBackupSchedule", func(t *testing.T) {
		t.Logf("backup schedule %q", comp.GetBackupSchedule())

		assert.NotEmpty(t, comp.GetBackupSchedule())
		backupSchedule := strings.Fields(comp.GetBackupSchedule())
		assert.Equal(t, "*", backupSchedule[4])
		assert.Equal(t, "*", backupSchedule[3])
		assert.Equal(t, "*", backupSchedule[2])

		backupMinute, err := strconv.Atoi(backupSchedule[0])
		assert.NoError(t, err)
		assert.Less(t, backupMinute, 60)
		assert.GreaterOrEqual(t, backupMinute, 0)
		backupHour, err := strconv.Atoi(backupSchedule[1])
		assert.Less(t, backupHour, 24)
		assert.GreaterOrEqual(t, backupHour, 0)
		assert.NoError(t, err)
		backupDay := 1
		if backupHour < 6 {
			backupDay = 2
		}
		backupTime = time.Date(0, 1, backupDay, backupHour, backupMinute, 0, 0, time.UTC)

		backupWindowStart := time.Date(0, 1, 1, 20, 0, 0, 0, time.UTC)
		backupWindowEnd := time.Date(0, 1, 2, 4, 0, 0, 0, time.UTC)
		assert.LessOrEqual(t, backupWindowStart, backupTime)
		assert.LessOrEqual(t, backupTime, backupWindowEnd)

	})
	return backupTime
}

func parseAndValidateMaitenance(t *testing.T, comp *vshnv1.VSHNPostgreSQL) time.Time {
	var maintTime time.Time
	t.Run("validateMaintenanceSchedule", func(t *testing.T) {

		t.Logf("maintenance time %q", comp.GetFullMaintenanceSchedule().TimeOfDay)
		t.Logf("maintenance day %q", comp.GetFullMaintenanceSchedule().DayOfWeek)

		var err error
		assert.NotEmpty(t, comp.GetFullMaintenanceSchedule().TimeOfDay)
		maintTime, err = time.ParseInLocation(time.TimeOnly, comp.GetFullMaintenanceSchedule().TimeOfDay, time.UTC)
		assert.NoError(t, err)
		assert.NotEmpty(t, comp.GetFullMaintenanceSchedule().DayOfWeek)
		switch comp.GetFullMaintenanceSchedule().DayOfWeek {
		case "tuesday":
		case "wednesday":
			maintTime = maintTime.Add(24 * time.Hour)
		default:
			assert.Failf(t, "unexpected Maintenance day", "Day: %q", comp.GetFullMaintenanceSchedule().DayOfWeek)
		}

		maintWindowStart := time.Date(0, 1, 1, 21, 0, 0, 0, time.UTC)
		maintWindowEnd := time.Date(0, 1, 2, 5, 0, 0, 0, time.UTC)
		assert.LessOrEqual(t, maintWindowStart, maintTime)
		assert.LessOrEqual(t, maintTime, maintWindowEnd)
	})
	return maintTime
}
