package common

import (
	"fmt"
	"math/rand"
	"time"

	v1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

var (
	maintenanceWindowStart = time.Date(1970, 1, 1, 21, 0, 0, 0, time.UTC)
	maintenanceWindowEnd   = time.Date(1970, 1, 2, 5, 0, 0, 0, time.UTC)
	maintenanceWindowRange = maintenanceWindowEnd.Unix() - maintenanceWindowStart.Unix()
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

// SetRandomSchedules initializes the backup and maintenance schedules if the user did not explicitly provide a schedule.
// The maintenance will be set to a random time on a random day (Sunday-Friday) between 21:00 and 5:00,
// with the exception that Sunday maintenance only runs after 21:00 (not in the early morning hours).
// The backup schedule will be set to once a day between 20:00 and 4:00.
// If neither maintenance nor backup is set, the function will make sure that there will be backup scheduled one hour before the maintenance.
func SetRandomSchedules(backup BackupScheduler, maintenance MaintenanceScheduler) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	availableDays := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday"}
	selectedDay := availableDays[rng.Intn(len(availableDays))]
	maintTime := time.Unix(maintenanceWindowStart.Unix()+rng.Int63n(maintenanceWindowRange), 0).In(time.UTC)

	// Special handling for Sunday: only allow times after 21:00 (not early morning)
	if selectedDay == "sunday" && maintTime.Hour() < 21 {
		// If time is in early morning (0-5), shift to evening (21-23)
		eveningStart := time.Date(1970, 1, 1, 21, 0, 0, 0, time.UTC)
		eveningEnd := time.Date(1970, 1, 1, 23, 59, 59, 0, time.UTC)
		eveningRange := eveningEnd.Unix() - eveningStart.Unix()
		maintTime = time.Unix(eveningStart.Unix()+rng.Int63n(eveningRange), 0).In(time.UTC)
	}

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
		maintenance.SetMaintenanceDayOfWeek(selectedDay)
	}
}
