package v1

import (
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnkafkastrimzis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnkafkastrimzis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnkafkastrimzis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnkafkastrimzis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.tls.default={})"

// +kubebuilder:object:root=true

// VSHNKafkaStrimzi is the API for creating KafkaStrimzi instances.
type VSHNKafkaStrimzi struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNKafkaStrimzi.
	Spec VSHNKafkaStrimziSpec `json:"spec"`

	// Status reflects the observed state of a VSHNKafkaStrimzi.
	Status VSHNKafkaStrimziStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type VSHNKafkaStrimziList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNKafkaStrimzi `json:"items,omitempty"`
}

// VSHNKafkaStrimziSpec defines the desired state of a VSHNKafkaStrimzi.
type VSHNKafkaStrimziSpec struct {
	// Parameters are the configurable fields of a VSHNKafkaStrimzi.
	Parameters VSHNKafkaStrimziParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

// VSHNKafkaStrimziParameters are the configurable fields of a VSHNKafkaStrimzi.
type VSHNKafkaStrimziParameters struct {
	// Service contains KafkaStrimzi DBaaS specific properties
	Service VSHNKafkaStrimziServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNSizeSpec `json:"size,omitempty"`

	// Scheduling contains settings to control the scheduling of an instance.
	Scheduling VSHNDBaaSSchedulingSpec `json:"scheduling,omitempty"`

	// TLS contains settings to control tls traffic of a service.
	TLS VSHNKafkaStrimziTLSSpec `json:"tls,omitempty"`

	// Backup contains settings to control how the instance should get backed up.
	Backup K8upBackupSpec `json:"backup,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance VSHNDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`

	// Monitoring contains settings to control monitoring.
	Monitoring VSHNMonitoring `json:"monitoring,omitempty"`

	// Security defines the security of a service
	Security *Security `json:"security,omitempty"`

	// TopologyKey defines the topology key for rack awareness
	TopologyKey string `json:"topologyKey,omitempty"`
}

// VSHNKafkaStrimziServiceSpec contains KafkaStrimzi DBaaS specific properties
type VSHNKafkaStrimziServiceSpec struct {
	// +kubebuilder:validation:Enum="0.49.0";"0.49.1";
	// +kubebuilder:default="0.49.1"

	// Version contains supported version of KafkaStrimzi.
	// Multiple versions are supported. The latest version 4.1 is the default version.
	Version string `json:"version,omitempty"`

	// KafkaStrimziSettings contains additional KafkaStrimzi settings.
	KafkaStrimziSettings string `json:"kafkaStrimziSettings,omitempty"`

	// +kubebuilder:validation:Enum="besteffort";"guaranteed"
	// +kubebuilder:default="besteffort"

	// ServiceLevel defines the service level of this service. Either Best Effort or Guaranteed Availability is allowed.
	ServiceLevel VSHNDBaaSServiceLevel `json:"serviceLevel,omitempty"`
}

// VSHNKafkaStrimziSizeSpec contains settings to control the sizing of a service.
type VSHNKafkaStrimziSizeSpec struct {

	// CPURequests defines the requests amount of Kubernetes CPUs for an instance.
	CPURequests string `json:"cpuRequests,omitempty"`

	// CPULimits defines the limits amount of Kubernetes CPUs for an instance.
	CPULimits string `json:"cpuLimits,omitempty"`

	// MemoryRequests defines the requests amount of memory in units of bytes for an instance.
	MemoryRequests string `json:"memoryRequests,omitempty"`

	// MemoryLimits defines the limits amount of memory in units of bytes for an instance.
	MemoryLimits string `json:"memoryLimits,omitempty"`

	// Disk defines the amount of disk space for an instance.
	Disk string `json:"disk,omitempty"`

	// Plan is the name of the resource plan that defines the compute resources.
	Plan string `json:"plan,omitempty"`
}

// VSHNKafkaStrimziTLSSpec contains settings to control tls traffic of a service.
type VSHNKafkaStrimziTLSSpec struct {
	// +kubebuilder:default=true

	// TLSEnabled enables TLS traffic for the service
	TLSEnabled bool `json:"enabled,omitempty"`

	// +kubebuilder:default=true
	// TLSAuthClients enables client authentication requirement
	TLSAuthClients bool `json:"authClients,omitempty"`
}

// VSHNKafkaStrimziStatus reflects the observed state of a VSHNKafkaStrimzi.
type VSHNKafkaStrimziStatus struct {
	NamespaceConditions []Condition `json:"namespaceConditions,omitempty"`
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
	// Schedules keeps track of random generated schedules, is overwriten by
	// schedules set in the service's spec.
	Schedules VSHNScheduleStatus `json:"schedules,omitempty"`
}

func (v *VSHNKafkaStrimzi) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

func (v *VSHNKafkaStrimzi) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-kafkastrimzi-%s", v.GetName())
}

func (v *VSHNKafkaStrimzi) SetInstanceNamespaceStatus() {
	v.Status.InstanceNamespace = v.GetInstanceNamespace()
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNKafkaStrimzi represents the internal composite of this claim
type XVSHNKafkaStrimzi struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XVSHNKafkaStrimziSpec   `json:"spec"`
	Status XVSHNKafkaStrimziStatus `json:"status,omitempty"`
}

// XVSHNKafkaStrimziSpec defines the desired state of a VSHNKafkaStrimzi.
type XVSHNKafkaStrimziSpec struct {
	// Parameters are the configurable fields of a VSHNKafkaStrimzi.
	Parameters VSHNKafkaStrimziParameters `json:"parameters,omitempty"`

	xpv1.ResourceSpec `json:",inline"`
}

type XVSHNKafkaStrimziStatus struct {
	VSHNKafkaStrimziStatus `json:",inline"`
	xpv1.ResourceStatus    `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNKafkaStrimziList represents a list of composites
type XVSHNKafkaStrimziList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XVSHNKafkaStrimzi `json:"items"`
}

// GetMaintenanceDayOfWeek returns the currently set day of week
func (v *VSHNKafkaStrimzi) GetMaintenanceDayOfWeek() string {
	if v.Spec.Parameters.Maintenance.DayOfWeek != "" {
		return v.Spec.Parameters.Maintenance.DayOfWeek
	}
	return v.Status.Schedules.Maintenance.DayOfWeek
}

// GetMaintenanceTimeOfDay returns the currently set time of day
func (v *VSHNKafkaStrimzi) GetMaintenanceTimeOfDay() TimeOfDay {
	if v.Spec.Parameters.Maintenance.TimeOfDay != "" {
		return v.Spec.Parameters.Maintenance.TimeOfDay
	}
	return v.Status.Schedules.Maintenance.TimeOfDay
}

// SetMaintenanceDayOfWeek sets the day of week to the given value
func (v *VSHNKafkaStrimzi) SetMaintenanceDayOfWeek(dow string) {
	v.Status.Schedules.Maintenance.DayOfWeek = dow
}

// SetMaintenanceTimeOfDay sets the time of day to the given value
func (v *VSHNKafkaStrimzi) SetMaintenanceTimeOfDay(tod TimeOfDay) {
	v.Status.Schedules.Maintenance.TimeOfDay = tod
}

// GetFullMaintenanceSchedule returns
func (v *VSHNKafkaStrimzi) GetFullMaintenanceSchedule() VSHNDBaaSMaintenanceScheduleSpec {
	schedule := v.Spec.Parameters.Maintenance
	schedule.DayOfWeek = v.GetMaintenanceDayOfWeek()
	schedule.TimeOfDay = v.GetMaintenanceTimeOfDay()
	return schedule
} // GetServiceName returns the name of this service
func (v *VSHNKafkaStrimzi) GetServiceName() string {
	return "kafkastrimzi"
}

// GetPDBLabels returns the labels to be used for the PodDisruptionBudget
// it should match one unique label od pod running in instanceNamespace
// without this, the PDB will match all pods
func (v *VSHNKafkaStrimzi) GetPDBLabels() map[string]string {
	return map[string]string{}
}

func (v *VSHNKafkaStrimzi) GetSize() VSHNSizeSpec {
	return v.Spec.Parameters.Size
}

func (v *VSHNKafkaStrimzi) GetBillingName() string {
	return "appcat-" + v.GetServiceName()
}

func (v *VSHNKafkaStrimzi) GetClaimName() string {
	return v.GetLabels()["crossplane.io/claim-name"]
}

func (v *VSHNKafkaStrimzi) GetSLA() string {
	return string(v.Spec.Parameters.Service.ServiceLevel)
}

func (v *VSHNKafkaStrimzi) GetWorkloadName() string {
	return v.GetName() + "-"
}

func (v *VSHNKafkaStrimzi) GetWorkloadPodTemplateLabelsManager() PodTemplateLabelsManager {
	return &StatefulSetManager{}
}

// GetBackupRetention returns the retention definition for this backup.
func (v *VSHNKafkaStrimzi) GetBackupRetention() K8upRetentionPolicy {
	return v.Spec.Parameters.Backup.Retention
}

// GetBackupSchedule returns the current backup schedule
func (v *VSHNKafkaStrimzi) GetBackupSchedule() string {
	if v.Spec.Parameters.Backup.Schedule != "" {
		return v.Spec.Parameters.Backup.Schedule
	}
	return v.Status.Schedules.Backup
}

// SetBackupSchedule overwrites the current backup schedule
func (v *VSHNKafkaStrimzi) SetBackupSchedule(schedule string) {
	v.Status.Schedules.Backup = schedule
}

// IsBackupEnabled returns true if backups are enabled
func (v *VSHNKafkaStrimzi) IsBackupEnabled() bool {
	if v.Spec.Parameters.Backup.Enabled != nil {
		return *v.Spec.Parameters.Backup.Enabled
	}
	return false
}

// GetInstances returns the number of instances
func (v *VSHNKafkaStrimzi) GetInstances() int {
	return 1
}

// GetMonitoring returns monitoring configuration
func (v *VSHNKafkaStrimzi) GetMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

// GetSecurity returns security configuration
func (v *VSHNKafkaStrimzi) GetSecurity() *Security {
	return v.Spec.Parameters.Security
}

// GetVSHNMonitoring returns monitoring configuration for alerting
func (v *VSHNKafkaStrimzi) GetVSHNMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

// GetTopologyKey returns the topology key for rack awareness
func (v *VSHNKafkaStrimzi) GetTopologyKey() string {
	return v.Spec.Parameters.TopologyKey
}

// GetAllowAllNamespaces returns whether to allow all namespaces
func (v *VSHNKafkaStrimzi) GetAllowAllNamespaces() bool {
	if v.Spec.Parameters.Security != nil {
		return v.Spec.Parameters.Security.AllowAllNamespaces
	}
	return false
}

// GetAllowedNamespaces returns the list of allowed namespaces
func (v *VSHNKafkaStrimzi) GetAllowedNamespaces() []string {
	if v.Spec.Parameters.Security != nil {
		return v.Spec.Parameters.Security.AllowedNamespaces
	}
	return []string{}
}
