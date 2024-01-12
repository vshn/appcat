package v1

import alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"

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

type VSHNDBaaSServiceLevel string

const (
	BestEffort VSHNDBaaSServiceLevel = "besteffort"
	Guaranteed VSHNDBaaSServiceLevel = "guaranteed"
)

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

// VSHNSizeSpec contains settings to control the sizing of a service.
type VSHNSizeSpec struct {
	// CPU defines the amount of Kubernetes CPUs for an instance.
	CPU string `json:"cpu,omitempty"`

	// Memory defines the amount of memory in units of bytes for an instance.
	Memory string `json:"memory,omitempty"`

	// Requests defines CPU and memory requests for an instance
	Requests VSHNDBaaSSizeRequestsSpec `json:"requests,omitempty"`

	// Disk defines the amount of disk space for an instance.
	Disk string `json:"disk,omitempty"`

	// Plan is the name of the resource plan that defines the compute resources.
	Plan string `json:"plan,omitempty"`
}

func (p *VSHNSizeSpec) GetPlan(defaultPlan string) string {
	if p.Plan != "" {
		return p.Plan
	}
	return defaultPlan
}

// VSHNDBaaSSizeRequestsSpec contains settings to control the resoure requests of a service.
type VSHNDBaaSSizeRequestsSpec struct {
	// CPU defines the amount of Kubernetes CPUs for an instance.
	CPU string `json:"cpu,omitempty"`

	// Memory defines the amount of memory in units of bytes for an instance.
	Memory string `json:"memory,omitempty"`
}

// VSHNDBaaSNetworkSpec contains any network related settings.
type VSHNDBaaSNetworkSpec struct {
	// +kubebuilder:default={"0.0.0.0/0"}

	// IPFilter is a list of allowed IPv4 CIDR ranges that can access the service.
	// If no IP Filter is set, you may not be able to reach the service.
	// A value of `0.0.0.0/0` will open the service to all addresses on the public internet.
	IPFilter []string `json:"ipFilter,omitempty"`

	// ServiceType defines the type of the service.
	// Possible enum values:
	//   - `"ClusterIP"` indicates that the service is only reachable from within the cluster.
	//   - `"LoadBalancer"` indicates that the service is reachable from the public internet via dedicated Ipv4 address.
	// +kubebuilder:default="ClusterIP"
	// +kubebuilder:validation:Enum="ClusterIP";"LoadBalancer"
	ServiceType string `json:"serviceType,omitempty"`
}

// VSHNDBaaSSchedulingSpec contains settings to control the scheduling of an instance.
type VSHNDBaaSSchedulingSpec struct {
	// NodeSelector is a selector which must match a node’s labels for the pod to be scheduled on that node
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// VSHNMonitoring contains settings to configure monitoring aspects of databases managed by VSHN
type VSHNMonitoring struct {
	// AlertmanagerConfigRef contains the name of the AlertmanagerConfig that should be copied over to the
	// namespace of the instance.
	AlertmanagerConfigRef string `json:"alertmanagerConfigRef,omitempty"`

	// AlertmanagerConfigSecretRef contains the name of the secret that is used
	// in the referenced AlertmanagerConfig
	AlertmanagerConfigSecretRef string `json:"alertmanagerConfigSecretRef,omitempty"`

	// AlertmanagerConfigSpecTemplate takes an AlertmanagerConfigSpec object.
	// This takes precedence over the AlertmanagerConfigRef.
	AlertmanagerConfigSpecTemplate *alertmanagerv1alpha1.AlertmanagerConfigSpec `json:"alertmanagerConfigTemplate,omitempty"`

	// Email necessary to send alerts via email
	Email string `json:"email,omitempty"`
	// VSHNScheduleStatus keeps track of the maintenance and backup schedules.
	// As of Crossplane 1.14 it's no longer allowed to change the composite.spec, so
	// any generate
}

type VSHNScheduleStatus struct {
	// Maintenance keeps track of the maintenance schedule.
	Maintenance VSHNDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`
	// Backup keeps track of the backup schedule.
	Backup string `json:"backup,omitempty"`
}
