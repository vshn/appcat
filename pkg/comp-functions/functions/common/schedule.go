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
	BackupEnabledChecker
}

// BackupEnabledChecker can check if backups are enabled
type BackupEnabledChecker interface {
	IsBackupEnabled() bool
}

type MaintenanceScheduler interface {
	GetMaintenanceDayOfWeek() string
	SetMaintenanceDayOfWeek(string)
	GetMaintenanceTimeOfDay() *v1.TimeOfDay
}

// SetRandomMaintenanceSchedule sets a random maintenance schedule if not already set.
// The maintenance will be set to a random time on a random day (Sunday-Friday) between 21:00 and 5:00,
// with the exception that Sunday maintenance only runs after 21:00 (not in the early morning hours).
// Returns the maintenance time that was set or already existed.
func SetRandomMaintenanceSchedule(maintenance MaintenanceScheduler) time.Time {
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

	timeOfDay := maintenance.GetMaintenanceTimeOfDay()
	if timeOfDay.IsNotSet() {
		timeOfDay.SetTime(maintTime)
	}

	if maintenance.GetMaintenanceDayOfWeek() == "" {
		maintenance.SetMaintenanceDayOfWeek(selectedDay)
	}

	return maintTime
}

// SetRandomBackupSchedule sets a random backup schedule if not already set and backups are enabled.
// The backup schedule will be set to once a day, one hour before the provided maintenance time.
// If no maintenance time is provided, it will be set to a random time between 20:00 and 4:00.
func SetRandomBackupSchedule(backup BackupScheduler, maintenanceTime *time.Time) {
	// Only set backup schedule if backups are enabled
	shouldSetBackupSchedule := backup.IsBackupEnabled()

	if !shouldSetBackupSchedule {
		// Clear backup schedule when backups are disabled
		if backup.GetBackupSchedule() != "" {
			backup.SetBackupSchedule("")
		}
		return
	}

	if backup.GetBackupSchedule() != "" {
		// Backup schedule already set
		return
	}

	var backupTime time.Time
	if maintenanceTime != nil {
		// Schedule backup one hour before maintenance
		backupTime = maintenanceTime.Add(-1 * time.Hour).In(time.UTC)
	} else {
		// Set random backup time between 20:00 and 4:00
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		backupWindowStart := time.Date(1970, 1, 1, 20, 0, 0, 0, time.UTC)
		backupWindowEnd := time.Date(1970, 1, 2, 4, 0, 0, 0, time.UTC)
		backupWindowRange := backupWindowEnd.Unix() - backupWindowStart.Unix()
		backupTime = time.Unix(backupWindowStart.Unix()+rng.Int63n(backupWindowRange), 0).In(time.UTC)
	}

	newSchedule := fmt.Sprintf("%d %d * * *", backupTime.Minute(), backupTime.Hour())
	backup.SetBackupSchedule(newSchedule)
}
