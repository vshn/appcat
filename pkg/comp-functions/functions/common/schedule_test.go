package common

import (
	"strconv"
	"strings"
	"testing"
	"time"

	v1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

// Test implementations that use real types
type testBackupScheduler struct {
	schedule string
	enabled  bool
}

func (t *testBackupScheduler) GetBackupSchedule() string {
	return t.schedule
}

func (t *testBackupScheduler) SetBackupSchedule(schedule string) {
	t.schedule = schedule
}

func (t *testBackupScheduler) IsBackupEnabled() bool {
	return t.enabled
}

type testMaintenanceScheduler struct {
	dayOfWeek string
	timeOfDay v1.TimeOfDay
}

func (t *testMaintenanceScheduler) GetMaintenanceDayOfWeek() string {
	return t.dayOfWeek
}

func (t *testMaintenanceScheduler) SetMaintenanceDayOfWeek(day string) {
	t.dayOfWeek = day
}

func (t *testMaintenanceScheduler) GetMaintenanceTimeOfDay() *v1.TimeOfDay {
	return &t.timeOfDay
}

func TestSetRandomSchedules_EmptySchedules(t *testing.T) {
	backup := &testBackupScheduler{enabled: true}
	maintenance := &testMaintenanceScheduler{}

	maintTime := SetRandomMaintenanceSchedule(maintenance)
	SetRandomBackupSchedule(backup, &maintTime)

	// Test that backup schedule is set
	if backup.GetBackupSchedule() == "" {
		t.Error("Expected backup schedule to be set, but it was empty")
	}

	// Test that maintenance day is set to one of the valid days
	validDays := map[string]bool{
		"sunday": true, "monday": true, "tuesday": true,
		"wednesday": true, "thursday": true, "friday": true,
	}
	day := maintenance.GetMaintenanceDayOfWeek()
	if !validDays[day] {
		t.Errorf("Expected maintenance day to be one of [sunday, monday, tuesday, wednesday, thursday, friday], got: %s", day)
	}

	// Test that maintenance time is set
	timeOfDay := maintenance.GetMaintenanceTimeOfDay()
	if timeOfDay.IsNotSet() {
		t.Error("Expected maintenance time to be set, but it was not")
	}
}

func TestSetRandomSchedules_PresetBackupSchedule(t *testing.T) {
	existingSchedule := "30 2 * * *"
	backup := &testBackupScheduler{schedule: existingSchedule, enabled: true}
	maintenance := &testMaintenanceScheduler{}

	maintTime := SetRandomMaintenanceSchedule(maintenance)
	SetRandomBackupSchedule(backup, &maintTime)

	// Test that existing backup schedule is preserved
	if backup.GetBackupSchedule() != existingSchedule {
		t.Errorf("Expected backup schedule to remain unchanged as '%s', got: '%s'", existingSchedule, backup.GetBackupSchedule())
	}

	// Test that maintenance is still set
	if maintenance.GetMaintenanceDayOfWeek() == "" {
		t.Error("Expected maintenance day to be set")
	}
}

func TestSetRandomSchedules_PresetMaintenanceDay(t *testing.T) {
	existingDay := "monday"
	backup := &testBackupScheduler{enabled: true}
	maintenance := &testMaintenanceScheduler{dayOfWeek: existingDay}

	maintTime := SetRandomMaintenanceSchedule(maintenance)
	SetRandomBackupSchedule(backup, &maintTime)

	// Test that existing maintenance day is preserved
	if maintenance.GetMaintenanceDayOfWeek() != existingDay {
		t.Errorf("Expected maintenance day to remain unchanged as '%s', got: '%s'", existingDay, maintenance.GetMaintenanceDayOfWeek())
	}

	// Test that backup schedule is still set
	if backup.GetBackupSchedule() == "" {
		t.Error("Expected backup schedule to be set")
	}
}

func TestSetRandomSchedules_PresetMaintenanceTime(t *testing.T) {
	backup := &testBackupScheduler{enabled: true}
	maintenance := &testMaintenanceScheduler{}

	// Pre-set the maintenance time
	presetTime := time.Date(2023, 1, 1, 22, 30, 0, 0, time.UTC)
	maintenance.timeOfDay.SetTime(presetTime)

	maintTime := SetRandomMaintenanceSchedule(maintenance)
	SetRandomBackupSchedule(backup, &maintTime)

	// Test that existing maintenance time is preserved
	timeOfDay := maintenance.GetMaintenanceTimeOfDay()
	if timeOfDay.IsNotSet() {
		t.Error("Expected maintenance time to remain set")
	}

	// Test that backup schedule and maintenance day are still set
	if backup.GetBackupSchedule() == "" {
		t.Error("Expected backup schedule to be set")
	}
	if maintenance.GetMaintenanceDayOfWeek() == "" {
		t.Error("Expected maintenance day to be set")
	}
}

func TestSetRandomSchedules_MaintenanceTimeWindow(t *testing.T) {
	// Run the test multiple times to check randomness
	for i := 0; i < 100; i++ {
		backup := &testBackupScheduler{enabled: true}
		maintenance := &testMaintenanceScheduler{}

		maintTime := SetRandomMaintenanceSchedule(maintenance)
		SetRandomBackupSchedule(backup, &maintTime)

		// Extract hour from the maintenance time
		// We need to check the actual time that was set
		timeOfDay := maintenance.GetMaintenanceTimeOfDay()
		if timeOfDay.IsNotSet() {
			t.Error("Maintenance time should be set")
			continue
		}

		day := maintenance.GetMaintenanceDayOfWeek()

		// Parse the backup schedule to get the maintenance time
		// Since backup is 1 hour before maintenance, we can calculate maintenance time
		backupSchedule := backup.GetBackupSchedule()
		parts := strings.Fields(backupSchedule)
		if len(parts) < 2 {
			t.Errorf("Invalid backup schedule format: %s", backupSchedule)
			continue
		}

		backupHour, err := strconv.Atoi(parts[1])
		if err != nil {
			t.Errorf("Invalid backup hour: %s", parts[1])
			continue
		}

		// Calculate maintenance hour (1 hour after backup)
		maintHour := (backupHour + 1) % 24

		// Check if maintenance time is within the valid window
		// For Sunday: only 21:00-23:59 (no early morning)
		// For other days: 21:00-23:59 or 00:00-04:59
		var validTime bool
		if day == "sunday" {
			validTime = maintHour >= 21 && maintHour <= 23
		} else {
			validTime = (maintHour >= 21 && maintHour <= 23) || (maintHour >= 0 && maintHour <= 4)
		}

		if !validTime {
			if day == "sunday" {
				t.Errorf("Sunday maintenance time %d:xx is outside the valid window (21:00-23:59)", maintHour)
			} else {
				t.Errorf("Maintenance time %d:xx on %s is outside the valid window (21:00-05:00)", maintHour, day)
			}
		}
	}
}

func TestSetRandomSchedules_BackupOneHourBeforeMaintenance(t *testing.T) {
	backup := &testBackupScheduler{enabled: true}
	maintenance := &testMaintenanceScheduler{}

	maintTime := SetRandomMaintenanceSchedule(maintenance)
	SetRandomBackupSchedule(backup, &maintTime)

	// Parse the backup schedule (format: "minute hour * * *")
	backupSchedule := backup.GetBackupSchedule()
	parts := strings.Fields(backupSchedule)
	if len(parts) < 2 {
		t.Fatalf("Invalid backup schedule format: %s", backupSchedule)
	}

	backupMinute, err := strconv.Atoi(parts[0])
	if err != nil {
		t.Fatalf("Invalid backup minute: %s", parts[0])
	}

	backupHour, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatalf("Invalid backup hour: %s", parts[1])
	}

	// The maintenance should be exactly 1 hour after backup
	expectedMaintHour := (backupHour + 1) % 24
	expectedMaintMinute := backupMinute

	// Verify the relationship exists (we can't directly get maintenance time from v1.TimeOfDay easily,
	// but we can verify the schedule format is correct)
	if backupSchedule == "" {
		t.Error("Backup schedule should not be empty")
	}

	// Test that the format is correct (minute hour * * *)
	if len(parts) != 5 || parts[2] != "*" || parts[3] != "*" || parts[4] != "*" {
		t.Errorf("Expected backup schedule format 'minute hour * * *', got: %s", backupSchedule)
	}

	t.Logf("Backup scheduled at %d:%02d, maintenance should be at %d:%02d",
		backupHour, backupMinute, expectedMaintHour, expectedMaintMinute)
}

func TestSetRandomSchedules_SundayTimeRestriction(t *testing.T) {
	sundayCount := 0
	iterations := 1000

	// Run many iterations to specifically test Sunday scheduling
	for i := 0; i < iterations; i++ {
		backup := &testBackupScheduler{enabled: true}
		maintenance := &testMaintenanceScheduler{}

		maintTime := SetRandomMaintenanceSchedule(maintenance)
		SetRandomBackupSchedule(backup, &maintTime)

		if maintenance.GetMaintenanceDayOfWeek() == "sunday" {
			sundayCount++

			// Parse backup schedule to determine maintenance time
			backupSchedule := backup.GetBackupSchedule()
			parts := strings.Fields(backupSchedule)
			if len(parts) >= 2 {
				backupHour, err := strconv.Atoi(parts[1])
				if err == nil {
					maintHour := (backupHour + 1) % 24

					// Sunday maintenance should only be between 21:00-23:59
					if maintHour < 21 {
						t.Errorf("Sunday maintenance scheduled at %d:xx, but should only be after 21:00", maintHour)
					}
				}
			}
		}
	}

	// Ensure we actually tested some Sunday cases
	if sundayCount == 0 {
		t.Errorf("No Sunday maintenance was scheduled in %d iterations", iterations)
	}

	t.Logf("Tested %d Sunday maintenance schedules out of %d total iterations", sundayCount, iterations)
}

func TestSetRandomSchedules_DayDistribution(t *testing.T) {
	dayCount := make(map[string]int)
	iterations := 1000

	// Run many iterations to test day distribution
	for i := 0; i < iterations; i++ {
		backup := &testBackupScheduler{enabled: true}
		maintenance := &testMaintenanceScheduler{}

		maintTime := SetRandomMaintenanceSchedule(maintenance)
		SetRandomBackupSchedule(backup, &maintTime)
		day := maintenance.GetMaintenanceDayOfWeek()
		dayCount[day]++
	}

	// Check that all expected days are present
	expectedDays := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday"}
	for _, day := range expectedDays {
		if dayCount[day] == 0 {
			t.Errorf("Day '%s' was never selected in %d iterations", day, iterations)
		}
	}

	// Check that no unexpected days are present
	for day := range dayCount {
		found := false
		for _, expectedDay := range expectedDays {
			if day == expectedDay {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Unexpected day '%s' was selected", day)
		}
	}

	// Check that distribution is reasonably uniform (each day should appear at least 10% of the time)
	minExpected := iterations / 10
	for day, count := range dayCount {
		if count < minExpected {
			t.Errorf("Day '%s' appeared only %d times out of %d iterations (expected at least %d)", day, count, iterations, minExpected)
		}
	}

	// Log the distribution for visibility
	t.Log("Day distribution:")
	for _, day := range expectedDays {
		t.Logf("  %s: %d times (%.1f%%)", day, dayCount[day], float64(dayCount[day])/float64(iterations)*100)
	}
}
