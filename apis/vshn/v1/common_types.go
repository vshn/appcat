package v1

// K8upBackupSpec specifies when a backup for redis should be triggered.
// It also contains the retention policy for the backup.
type K8upBackupSpec struct {
	// +kubebuilder:validation:Pattern=^(\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([0-9]|1[0-9]|2[0-3])|\*\/([0-9]|1[0-9]|2[0-3])) (\*|([1-9]|1[0-9]|2[0-9]|3[0-1])|\*\/([1-9]|1[0-9]|2[0-9]|3[0-1])) (\*|([1-9]|1[0-2])|\*\/([1-9]|1[0-2])) (\*|([0-6])|\*\/([0-6]))$
	Schedule string `json:"schedule,omitempty"`

	Retention K8upRetentionPolicy `json:"retention,omitempty"`
}

// GetBackupSchedule returns the currently set schedule for this backup config
func (k *K8upBackupSpec) GetBackupSchedule() string {
	return k.Schedule
}

// SetBackupSchedule sets the schedule to the given value
func (k *K8upBackupSpec) SetBackupSchedule(schedule string) {
	k.Schedule = schedule
}

// K8upRetentionPolicy describes the retention configuration for a K8up backup.
type K8upRetentionPolicy struct {
	KeepLast   int `json:"keepLast,omitempty"`
	KeepHourly int `json:"keepHourly,omitempty"`
	// +kubebuilder:default=6
	KeepDaily   int `json:"keepDaily,omitempty"`
	KeepWeekly  int `json:"keepWeekly,omitempty"`
	KeepMonthly int `json:"keepMonthly,omitempty"`
	KeepYearly  int `json:"keepYearly,omitempty"`
}

// K8upRestoreSpec contains restore specific parameters.
type K8upRestoreSpec struct {

	// ClaimName specifies the name of the instance you want to restore from.
	// The claim has to be in the same namespace as this new instance.
	ClaimName string `json:"claimName,omitempty"`

	// BackupName is the name of the specific backup you want to restore.
	BackupName string `json:"backupName,omitempty"`
}

// VSHNDBaaSMaintenanceScheduleSpec contains settings to control the maintenance of an instance.
type VSHNDBaaSMaintenanceScheduleSpec struct {
	// +kubebuilder:validation:Enum=monday;tuesday;wednesday;thursday;friday;saturday;sunday

	// DayOfWeek specifies at which weekday the maintenance is held place.
	// Allowed values are [monday, tuesday, wednesday, thursday, friday, saturday, sunday]
	DayOfWeek string `json:"dayOfWeek,omitempty"`

	// +kubebuilder:validation:Pattern="^([0-1]?[0-9]|2[0-3]):([0-5][0-9]):([0-5][0-9])$"

	// TimeOfDay for installing updates in UTC.
	// Format: "hh:mm:ss".
	TimeOfDay string `json:"timeOfDay,omitempty"`
}

// GetMaintenanceDayOfWeek returns the currently set day of week
func (n *VSHNDBaaSMaintenanceScheduleSpec) GetMaintenanceDayOfWeek() string {
	return n.DayOfWeek
}

// GetMaintenanceTimeOfDay returns the currently set time of day
func (n *VSHNDBaaSMaintenanceScheduleSpec) GetMaintenanceTimeOfDay() string {
	return n.TimeOfDay
}

// SetMaintenanceDayOfWeek sets the day of week to the given value
func (n *VSHNDBaaSMaintenanceScheduleSpec) SetMaintenanceDayOfWeek(dow string) {
	n.DayOfWeek = dow
}

// SetMaintenanceTimeOfDay sets the time of day to the given value
func (n *VSHNDBaaSMaintenanceScheduleSpec) SetMaintenanceTimeOfDay(tod string) {
	n.TimeOfDay = tod
}
