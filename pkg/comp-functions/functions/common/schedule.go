package common

import (
	"fmt"
	"math/rand"
	"time"

	v1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

var (
	maitenanceWindowStart = time.Date(1970, 1, 1, 21, 0, 0, 0, time.UTC)
	maitenanceWindowEnd   = time.Date(1970, 1, 2, 5, 0, 0, 0, time.UTC)
	maitenanceWindowRange = maitenanceWindowEnd.Unix() - maitenanceWindowStart.Unix()
)

// BackupScheduler can schedule backups
type BackupScheduler interface {
	GetBackupSchedule() string
	SetBackupSchedule(string)
}

type MaintenanceScheduler interface {
	GetMaintenanceDayOfWeek() string
	SetMaintenanceDayOfWeek(string)
	GetMaintenanceTimeOfDay() *v1.TimeOfDay
}

// SetRandomSchedules initializes the backup and maintenance schedules  if the user did not explicitly provide a schedule.
// The maintenance will be set to a random time on Tuesday night between 21:00 and 5:00, and the backup schedule will be set to once a day between 20:00 and 4:00.
// If neither maintenance nor backup is set, the function will make sure that there will be backup scheduled one hour before the maintenance.
func SetRandomSchedules(backup BackupScheduler, maintenance MaintenanceScheduler) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	maintTime := time.Unix(maitenanceWindowStart.Unix()+rng.Int63n(maitenanceWindowRange), 0).In(time.UTC)
	backupTime := maintTime.Add(-1 * time.Hour).In(time.UTC)

	if backup.GetBackupSchedule() == "" {
		newSchedule := fmt.Sprintf("%d %d * * *", backupTime.Minute(), backupTime.Hour())
		backup.SetBackupSchedule(newSchedule)
	}

	timeOfDay := maintenance.GetMaintenanceTimeOfDay()
	if timeOfDay.IsNotSet() {
		timeOfDay.SetTime(maintTime)
	}

	if maintenance.GetMaintenanceDayOfWeek() == "" {
		day := "tuesday"
		if maintTime.Day() > 1 {
			day = "wednesday"
		}
		maintenance.SetMaintenanceDayOfWeek(day)
	}
}
