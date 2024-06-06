package v1

import (
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	v1 "github.com/vshn/appcat/v4/apis/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnforgejos.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnforgejos.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnforgejos.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnforgejos.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.tls.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnforgejos.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.default={})"

// +kubebuilder:object:root=true

// VSHNForgejo is the API for creating forgejo instances.
type VSHNForgejo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNForgejo.
	Spec VSHNForgejoSpec `json:"spec"`

	// Status reflects the observed state of a VSHNForgejo.
	Status VSHNForgejoStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type VSHNForgejoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNForgejo `json:"items,omitempty"`
}

// VSHNForgejoSpec defines the desired state of a VSHNForgejo.
type VSHNForgejoSpec struct {
	// Parameters are the configurable fields of a VSHNForgejo.
	Parameters VSHNForgejoParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef v1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

// VSHNForgejoParameters are the configurable fields of a VSHNForgejo.
type VSHNForgejoParameters struct {
	// Service contains forgejo DBaaS specific properties
	Service VSHNForgejoServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNSizeSpec `json:"size,omitempty"`

	// Scheduling contains settings to control the scheduling of an instance.
	Scheduling VSHNDBaaSSchedulingSpec `json:"scheduling,omitempty"`

	// TLS contains settings to control tls traffic of a service.
	TLS VSHNForgejoTLSSpec `json:"tls,omitempty"`

	// Backup contains settings to control how the instance should get backed up.
	Backup K8upBackupSpec `json:"backup,omitempty"`

	// Restore contains settings to control the restore of an instance.
	Restore K8upRestoreSpec `json:"restore,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance VSHNDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`

	// Security defines the security of a service
	Security Security `json:"security,omitempty"`
}

// VSHNForgejoServiceSpec contains forgejo DBaaS specific properties
type VSHNForgejoServiceSpec struct {
	// +kubebuilder:validation:Enum=<TBD>
	// +kubebuilder:default=<TBD>

	// Version contains supported version of forgejo.
	// Multiple versions are supported. The latest version <TBD> is the default version.
	Version string `json:"version,omitempty"`

	// Forgejosettings contains additional forgejo settings.
	Forgejosettings string `json:"forgejosettings,omitempty"`

	// +kubebuilder:validation:Enum="besteffort";"guaranteed"
	// +kubebuilder:default="besteffort"

	// ServiceLevel defines the service level of this service. Either Best Effort or Guaranteed Availability is allowed.
	ServiceLevel VSHNDBaaSServiceLevel `json:"serviceLevel,omitempty"`
}

// VSHNForgejoSizeSpec contains settings to control the sizing of a service.
type VSHNForgejoSizeSpec struct {

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

// VSHNForgejoTLSSpec contains settings to control tls traffic of a service.
type VSHNForgejoTLSSpec struct {
	// +kubebuilder:default=true

	// TLSEnabled enables TLS traffic for the service
	TLSEnabled bool `json:"enabled,omitempty"`

	// +kubebuilder:default=true
	// TLSAuthClients enables client authentication requirement
	TLSAuthClients bool `json:"authClients,omitempty"`
}

// VSHNForgejoStatus reflects the observed state of a VSHNForgejo.
type VSHNForgejoStatus struct {
	NamespaceConditions         []v1.Condition `json:"namespaceConditions,omitempty"`
	SelfSignedIssuerConditions  []v1.Condition `json:"selfSignedIssuerConditions,omitempty"`
	LocalCAConditions           []v1.Condition `json:"localCAConditions,omitempty"`
	CaCertificateConditions     []v1.Condition `json:"caCertificateConditions,omitempty"`
	ServerCertificateConditions []v1.Condition `json:"serverCertificateConditions,omitempty"`
	ClientCertificateConditions []v1.Condition `json:"clientCertificateConditions,omitempty"`
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
	// Schedules keeps track of random generated schedules, is overwriten by
	// schedules set in the service's spec.
	Schedules VSHNScheduleStatus `json:"schedules,omitempty"`
}

func (v *VSHNForgejo) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

func (v *VSHNForgejo) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-forgejo-%s", v.GetName())
}

func (v *VSHNForgejo) SetInstanceNamespaceStatus() {
	v.Status.InstanceNamespace = v.GetInstanceNamespace()
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNForgejo represents the internal composite of this claim
type XVSHNForgejo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XVSHNForgejoSpec   `json:"spec"`
	Status XVSHNForgejoStatus `json:"status,omitempty"`
}

// XVSHNForgejoSpec defines the desired state of a VSHNForgejo.
type XVSHNForgejoSpec struct {
	// Parameters are the configurable fields of a VSHNForgejo.
	Parameters VSHNForgejoParameters `json:"parameters,omitempty"`

	xpv1.ResourceSpec `json:",inline"`
}

type XVSHNForgejoStatus struct {
	VSHNForgejoStatus   `json:",inline"`
	xpv1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNForgejoList represents a list of composites
type XVSHNForgejoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XVSHNForgejo `json:"items"`
}

// GetMaintenanceDayOfWeek returns the currently set day of week
func (v *VSHNForgejo) GetMaintenanceDayOfWeek() string {
	if v.Spec.Parameters.Maintenance.DayOfWeek != "" {
		return v.Spec.Parameters.Maintenance.DayOfWeek
	}
	return v.Status.Schedules.Maintenance.DayOfWeek
}

// GetMaintenanceTimeOfDay returns the currently set time of day
func (v *VSHNForgejo) GetMaintenanceTimeOfDay() string {
	if v.Spec.Parameters.Maintenance.TimeOfDay != "" {
		return v.Spec.Parameters.Maintenance.TimeOfDay
	}
	return v.Status.Schedules.Maintenance.TimeOfDay
}

// SetMaintenanceDayOfWeek sets the day of week to the given value
func (v *VSHNForgejo) SetMaintenanceDayOfWeek(dow string) {
	v.Status.Schedules.Maintenance.DayOfWeek = dow
}

// SetMaintenanceTimeOfDay sets the time of day to the given value
func (v *VSHNForgejo) SetMaintenanceTimeOfDay(tod string) {
	v.Status.Schedules.Maintenance.TimeOfDay = tod
}

// GetBackupSchedule returns the current backup schedule
func (v *VSHNForgejo) GetBackupSchedule() string {
	if v.Spec.Parameters.Backup.Schedule != "" {
		return v.Spec.Parameters.Backup.Schedule
	}
	return v.Status.Schedules.Backup
}

// SetBackupSchedule overwrites the current backup schedule
func (v *VSHNForgejo) SetBackupSchedule(schedule string) {
	v.Status.Schedules.Backup = schedule
}

// GetBackupRetention returns the retention definition for this backup.
func (v *VSHNForgejo) GetBackupRetention() K8upRetentionPolicy {
	return v.Spec.Parameters.Backup.Retention
}

// GetServiceName returns the name of this service
func (v *VSHNForgejo) GetServiceName() string {
	return "forgejo"
}

// GetFullMaintenanceSchedule returns
func (v *VSHNForgejo) GetFullMaintenanceSchedule() VSHNDBaaSMaintenanceScheduleSpec {
	schedule := v.Spec.Parameters.Maintenance
	schedule.DayOfWeek = v.GetMaintenanceDayOfWeek()
	schedule.TimeOfDay = v.GetMaintenanceTimeOfDay()
	return schedule
}

// GetAllowAllNamespaces returns the AllowAllNamespaces field of this service
func (v *VSHNForgejo) GetAllowAllNamespaces() bool {
	return v.Spec.Parameters.Security.AllowAllNamespaces
}

// GetAllowedNamespaces returns the AllowedNamespaces array of this service
func (v *VSHNForgejo) GetAllowedNamespaces() []string {
	if v.Spec.Parameters.Security.AllowedNamespaces == nil {
		v.Spec.Parameters.Security.AllowedNamespaces = []string{}
	}
	return append(v.Spec.Parameters.Security.AllowedNamespaces, v.GetClaimNamespace())
}
