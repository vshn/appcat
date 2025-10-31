package v1

import (
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnopenbaoes.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnopenbaoes.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnopenbaoes.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnopenbaoes.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.default={})"

//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnopenbaoes.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.security.default={})"

// +kubebuilder:object:root=true

// VSHNOpenBao is the API for creating OpenBao instances.
type VSHNOpenBao struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNOpenBao.
	Spec VSHNOpenBaoSpec `json:"spec"`

	// Status reflects the observed state of a VSHNOpenBao.
	Status VSHNOpenBaoStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type VSHNOpenBaoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNOpenBao `json:"items,omitempty"`
}

// VSHNOpenBaoSpec defines the desired state of a VSHNOpenBao.
type VSHNOpenBaoSpec struct {
	// Parameters are the configurable fields of a VSHNOpenBao.
	Parameters VSHNOpenBaoParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

// VSHNOpenBaoParameters are the configurable fields of a VSHNOpenBao.
type VSHNOpenBaoParameters struct {
	// Service contains OpenBao DBaaS specific properties
	Service VSHNOpenBaoServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNSizeSpec `json:"size,omitempty"`

	// Scheduling contains settings to control the scheduling of an instance.
	Scheduling VSHNDBaaSSchedulingSpec `json:"scheduling,omitempty"`

	// Backup contains settings to control how the instance should get backed up.
	Backup K8upBackupSpec `json:"backup,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance VSHNDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`

	// Security contains settings to control the security of a service.
	Security Security `json:"security,omitempty"`

	// Monitoring contains settings to control the monitoring of a service.
	Monitoring VSHNMonitoring `json:"monitoring,omitempty"`

	// Instances defines the number of instances to run.
	Instances int `json:"instances,omitempty"`
}

// VSHNOpenBaoServiceSpec contains OpenBao specific properties
type VSHNOpenBaoServiceSpec struct {
	// +kubebuilder:validation:Enum=<TBD>
	// +kubebuilder:default=<TBD>

	// Version contains supported version of OpenBao.
	// Multiple versions are supported. The latest version <TBD> is the default version.
	Version string `json:"version,omitempty"`

	// OpenBaoSettings contains additional OpenBao settings.
	OpenBaoSettings VSHNOpenBaoSettings `json:"openBaoSettings,omitempty"`

	// +kubebuilder:validation:Enum="besteffort";"guaranteed"
	// +kubebuilder:default="besteffort"

	// ServiceLevel defines the service level of this service. Either Best Effort or Guaranteed Availability is allowed.
	ServiceLevel VSHNDBaaSServiceLevel `json:"serviceLevel,omitempty"`
}

// VSHNOpenBaoSettings contains OpenBao settings
type VSHNOpenBaoSettings struct {
	// AutoUnseal configures various auto unseal methods.
	AutoUnseal VSHNOpenBaoSettingsAutoUnseal `json:"version,omitempty"`
}

// VSHNOpenBaoSettingsAutoUnseal contains OpenBao auto-unseal configuration
type VSHNOpenBaoSettingsAutoUnseal struct {
	// AWSKmsSecretRef references to secret containing AWS KMS credentials and configuration
	AWSKmsSecretRef LocalObjectReference `json:"awsKmsSecretRef,omitempty"`
	// AzureKeyVaultSecretRef references to secret containing Azure Key Vault credentials and configuration
	AzureKeyVaultSecretRef LocalObjectReference `json:"azureKeyVaultSecretRef,omitempty"`
	// GCPKmsSecretRef references to secret containing GCP KMS credentials and configuration
	GCPKmsSecretRef LocalObjectReference `json:"gcpKmsSecretRef,omitempty"`
	// TransitSecretRef references to secret containing Transit auto-unseal configuration
	TransitSecretRef LocalObjectReference `json:"transitSecretRef,omitempty"`
}

// VSHNOpenBaoSizeSpec contains settings to control the sizing of a service.
type VSHNOpenBaoSizeSpec struct {

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

// VSHNOpenBaoStatus reflects the observed state of a VSHNOpenBao.
type VSHNOpenBaoStatus struct {
	NamespaceConditions         []Condition `json:"namespaceConditions,omitempty"`
	SelfSignedIssuerConditions  []Condition `json:"selfSignedIssuerConditions,omitempty"`
	LocalCAConditions           []Condition `json:"localCAConditions,omitempty"`
	CaCertificateConditions     []Condition `json:"caCertificateConditions,omitempty"`
	ServerCertificateConditions []Condition `json:"serverCertificateConditions,omitempty"`
	ClientCertificateConditions []Condition `json:"clientCertificateConditions,omitempty"`
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
	// Schedules keeps track of random generated schedules, is overwriten by
	// schedules set in the service's spec.
	Schedules VSHNScheduleStatus `json:"schedules,omitempty"`
}

func (v *VSHNOpenBao) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

func (v *VSHNOpenBao) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-openbao-%s", v.GetName())
}

func (v *VSHNOpenBao) SetInstanceNamespaceStatus() {
	v.Status.InstanceNamespace = v.GetInstanceNamespace()
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNOpenBao represents the internal composite of this claim
type XVSHNOpenBao struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XVSHNOpenBaoSpec   `json:"spec"`
	Status XVSHNOpenBaoStatus `json:"status,omitempty"`
}

// XVSHNOpenBaoSpec defines the desired state of a VSHNOpenBao.
type XVSHNOpenBaoSpec struct {
	// Parameters are the configurable fields of a VSHNOpenBao.
	Parameters VSHNOpenBaoParameters `json:"parameters,omitempty"`

	xpv1.ResourceSpec `json:",inline"`
}

type XVSHNOpenBaoStatus struct {
	VSHNOpenBaoStatus   `json:",inline"`
	xpv1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNOpenBaoList represents a list of composites
type XVSHNOpenBaoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XVSHNOpenBao `json:"items"`
}

// GetMaintenanceDayOfWeek returns the currently set day of week
func (v *VSHNOpenBao) GetMaintenanceDayOfWeek() string {
	if v.Spec.Parameters.Maintenance.DayOfWeek != "" {
		return v.Spec.Parameters.Maintenance.DayOfWeek
	}
	return v.Status.Schedules.Maintenance.DayOfWeek
}

// GetMaintenanceTimeOfDay returns the currently set time of day
func (v *VSHNOpenBao) GetMaintenanceTimeOfDay() TimeOfDay {
	if v.Spec.Parameters.Maintenance.TimeOfDay != "" {
		return v.Spec.Parameters.Maintenance.TimeOfDay
	}
	return v.Status.Schedules.Maintenance.TimeOfDay
}

// SetMaintenanceDayOfWeek sets the day of week to the given value
func (v *VSHNOpenBao) SetMaintenanceDayOfWeek(dow string) {
	v.Status.Schedules.Maintenance.DayOfWeek = dow
}

// SetMaintenanceTimeOfDay sets the time of day to the given value
func (v *VSHNOpenBao) SetMaintenanceTimeOfDay(tod TimeOfDay) {
	v.Status.Schedules.Maintenance.TimeOfDay = tod
}

// GetFullMaintenanceSchedule returns
func (v *VSHNOpenBao) GetFullMaintenanceSchedule() VSHNDBaaSMaintenanceScheduleSpec {
	schedule := v.Spec.Parameters.Maintenance
	schedule.DayOfWeek = v.GetMaintenanceDayOfWeek()
	schedule.TimeOfDay = v.GetMaintenanceTimeOfDay()
	return schedule
}

// GetBackupRetention returns the retention definition for this backup.
func (v *VSHNOpenBao) GetBackupRetention() K8upRetentionPolicy {
	return v.Spec.Parameters.Backup.Retention
}

// GetBackupSchedule returns the current backup schedule
func (v *VSHNOpenBao) GetBackupSchedule() string {
	if v.Spec.Parameters.Backup.Schedule != "" {
		return v.Spec.Parameters.Backup.Schedule
	}
	return v.Status.Schedules.Backup
}

// SetBackupSchedule overwrites the current backup schedule
func (v *VSHNOpenBao) SetBackupSchedule(schedule string) {
	v.Status.Schedules.Backup = schedule
} // GetServiceName returns the name of this service
func (v *VSHNOpenBao) GetServiceName() string {
	return "openbao"
}

// GetPDBLabels returns the labels to be used for the PodDisruptionBudget
// it should match one unique label od pod running in instanceNamespace
// without this, the PDB will match all pods
func (v *VSHNOpenBao) GetPDBLabels() map[string]string {
	return map[string]string{}
}

// GetAllowAllNamespaces returns the AllowAllNamespaces field of this service
func (v *VSHNOpenBao) GetAllowAllNamespaces() bool {
	return v.Spec.Parameters.Security.AllowAllNamespaces
}

// GetAllowedNamespaces returns the AllowedNamespaces array of this service
func (v *VSHNOpenBao) GetAllowedNamespaces() []string {
	if v.Spec.Parameters.Security.AllowedNamespaces == nil {
		v.Spec.Parameters.Security.AllowedNamespaces = []string{}
	}
	return append(v.Spec.Parameters.Security.AllowedNamespaces, v.GetClaimNamespace())
}

func (v *VSHNOpenBao) GetSecurity() *Security {
	return &v.Spec.Parameters.Security
}

func (v *VSHNOpenBao) GetSize() VSHNSizeSpec {
	return v.Spec.Parameters.Size
}

func (v *VSHNOpenBao) GetMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

func (v *VSHNOpenBao) GetInstances() int {
	return v.Spec.Parameters.Instances
}

func (v *VSHNOpenBao) GetBillingName() string {
	return "appcat-" + v.GetServiceName()
}

func (v *VSHNOpenBao) GetClaimName() string {
	return v.GetLabels()["crossplane.io/claim-name"]
}

func (v *VSHNOpenBao) GetSLA() string {
	return string(v.Spec.Parameters.Service.ServiceLevel)
}

func (v *VSHNOpenBao) GetWorkloadName() string {
	return v.GetName() + "-openBaoDeployment"
}

func (v *VSHNOpenBao) GetWorkloadPodTemplateLabelsManager() PodTemplateLabelsManager {
	return &StatefulSetManager{}
}

// IsBackupEnabled returns true if backups are enabled for this instance
// MinIO doesn't currently support backups via K8up, so this always returns false
func (v *VSHNOpenBao) IsBackupEnabled() bool {
	return false
}
