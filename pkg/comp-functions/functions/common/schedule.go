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

type Scheduler interface {
	BackupScheduler
	MaintenanceScheduler
}

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

// Checks if user has provided a maintenance schedule and randomly generates one if not.
// The schedule will occur at a random time on Tuesday night between 21:00 and 05:00.
func EnsureMaintenanceSchedule(maintenance MaintenanceScheduler) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	maintTime := time.Unix(maitenanceWindowStart.Unix()+rng.Int63n(maitenanceWindowRange), 0).In(time.UTC)

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

// Checks if user has provided a backup schedule and randomly generates one if not.
// The schedule will occur once a day between 20:00 and 04:00 - i.e. one hour before the maintenance time.
// Will return an error if neither a maintenance nor a backup schedule is defined.
func EnsureBackupSchedule(scheduler Scheduler) error {
	maintTime := scheduler.GetMaintenanceTimeOfDay()

	if scheduler.GetBackupSchedule() == "" {
		if maintTime.IsNotSet() {
			return fmt.Errorf("cannot generate schedule: no maintenance schedule defined")
		}

		backupTime := maintTime.GetTime().Add(-1 * time.Hour).In(time.UTC)
		newSchedule := fmt.Sprintf("%d %d * * *", backupTime.Minute(), backupTime.Hour())
		scheduler.SetBackupSchedule(newSchedule)
	}

	return nil
}
