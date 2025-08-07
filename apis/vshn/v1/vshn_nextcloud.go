package v1

import (
	"fmt"
	"strings"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnnextclouds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnnextclouds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnnextclouds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.properties.postgreSQLParameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnnextclouds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.properties.postgreSQLParameters.properties.service.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnnextclouds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.properties.postgreSQLParameters.properties.service.properties.tls.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnnextclouds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnnextclouds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.properties.retention.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnnextclouds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.security.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnnextclouds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.collabora.default={})"

// +kubebuilder:object:root=true

// VSHNNextcloud is the API for creating nextcloud instances.
type VSHNNextcloud struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNNextcloud.
	Spec VSHNNextcloudSpec `json:"spec"`

	// Status reflects the observed state of a VSHNNextcloud.
	Status VSHNNextcloudStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type VSHNNextcloudList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNNextcloud `json:"items,omitempty"`
}

// VSHNNextcloudSpec defines the desired state of a VSHNNextcloud.
type VSHNNextcloudSpec struct {
	// Parameters are the configurable fields of a VSHNNextcloud.
	Parameters VSHNNextcloudParameters `json:"parameters,omitempty"`

	// ResourceRef tracks the internal composite belonging to this claim
	ResourceRef xpv1.TypedReference `json:"resourceRef,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

type VSHNNextcloudBackupSpec struct {
	K8upBackupSpec `json:",inline"`

	// +kubebuilder:default=false

	// SkipMaintenance defines, if setting maintenance mode should be skipped during the backup. Defaults to false
	// Warning: If this is set to true, the maintenance mode will not be enabled during the backup. This might
	// lead to inconsistent backups.
	SkipMaintenance bool `json:"skipMaintenance,omitempty"`
}

// VSHNNextcloudParameters are the configurable fields of a VSHNNextcloud.
type VSHNNextcloudParameters struct {
	// Service contains nextcloud DBaaS specific properties
	Service VSHNNextcloudServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNSizeSpec `json:"size,omitempty"`

	// Scheduling contains settings to control the scheduling of an instance.
	Scheduling VSHNDBaaSSchedulingSpec `json:"scheduling,omitempty"`

	// Backup contains settings to control how the instance should get backed up.
	Backup VSHNNextcloudBackupSpec `json:"backup,omitempty"`

	// Restore contains settings to control the restore of an instance.
	Restore K8upRestoreSpec `json:"restore,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance VSHNDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`

	// Monitoring contains settings to control monitoring.
	Monitoring VSHNMonitoring `json:"monitoring,omitempty"`

	// Security defines the security of a service
	Security Security `json:"security,omitempty"`

	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3

	// Instances configures the number of Nextcloud instances for the cluster.
	// Each instance contains one Nextcloud server.
	Instances int `json:"instances,omitempty"`
}

// VSHNNextcloudServiceSpec contains nextcloud DBaaS specific properties
type VSHNNextcloudServiceSpec struct {
	// Collabora contains settings to control the Collabora integration.
	Collabora CollaboraSpec `json:"collabora,omitempty"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1

	// FQDN contains the FQDNs array, which will be used for the ingress.
	// If it's not set, no ingress will be deployed.
	// This also enables strict hostname checking for this FQDN.
	FQDN []string `json:"fqdn"`

	// RelativePath on which Nextcloud will listen.
	// +kubebuilder:default="/"
	RelativePath string `json:"relativePath,omitempty"`

	// +kubebuilder:default="31"

	// Version contains supported version of nextcloud.
	// Multiple versions are supported. The latest version 31 is the default version.
	Version string `json:"version,omitempty"`

	// +kubebuilder:validation:Enum="besteffort";"guaranteed"
	// +kubebuilder:default="besteffort"

	// ServiceLevel defines the service level of this service. Either Best Effort or Guaranteed Availability is allowed.
	ServiceLevel VSHNDBaaSServiceLevel `json:"serviceLevel,omitempty"`

	// +kubebuilder:default=true

	// UseExternalPostgreSQL defines if the VSHNPostgreSQL database backend should be used. Defaults to true. If set to false,
	// the build-in SQLite database is being used.
	UseExternalPostgreSQL bool `json:"useExternalPostgreSQL,omitempty"`

	// ExistingPGConnectionSecret uses a connection secret from an existing VSHNPostgreSQL instance.
	// It is recommended to use a custom user with a separate database from vshnpostgresql.spec.service.access.
	// This parameter will take precedence over PostgreSQLParameters
	// Changing databases for an existing Nextcloud instance can corrupt the service.
	ExistingPGConnectionSecret string `json:"existingPGConnectionSecret,omitempty"`

	// PostgreSQLParameters can be used to set any supported setting in the
	// underlying PostgreSQL instance.
	PostgreSQLParameters *VSHNPostgreSQLParameters `json:"postgreSQLParameters,omitempty"`
}

// VSHNNextcloudSettings contains Nextcloud specific settings.
type VSHNNextcloudSettings struct{}

// VSHNNextcloudSizeSpec contains settings to control the sizing of a service.
type VSHNNextcloudSizeSpec struct {

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

// VSHNNextcloudStatus reflects the observed state of a VSHNNextcloud.
type VSHNNextcloudStatus struct {
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
	// Schedules keeps track of random generated schedules, is overwriten by
	// schedules set in the service's spec.
	Schedules VSHNScheduleStatus `json:"schedules,omitempty"`
	// ResourceStatus represents the observed state of a managed resource.
	xpv1.ResourceStatus `json:",inline"`
}

func (v *VSHNNextcloud) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

func (v *VSHNNextcloud) GetClaimName() string {
	return v.GetLabels()["crossplane.io/claim-name"]
}

func (v *VSHNNextcloud) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-nextcloud-%s", v.GetName())
}

func (v *XVSHNNextcloud) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-nextcloud-%s", v.GetName())
}

func (v *VSHNNextcloud) SetInstanceNamespaceStatus() {
	v.Status.InstanceNamespace = v.GetInstanceNamespace()
}

// CollaboraSpec defines the desired state of a Collabora instance.
type CollaboraSpec struct {
	// Enabled enables the Collabora integration. It will autoconfigure the Collabora server URL in Your Nextcloud instance.
	//+kubebuilder:default=false
	Enabled bool `json:"enabled"`
	// FQDN contains the FQDN of the Collabora server. This is used to configure the Collabora server URL in Your Nextcloud instance.
	//+kubebuilder:validation:Required
	FQDN string `json:"fqdn,omitempty"`
	// Version defines the Collabora version to use.
	Version string `json:"version,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNNextcloud represents the internal composite of this claim
type XVSHNNextcloud struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XVSHNNextcloudSpec   `json:"spec"`
	Status XVSHNNextcloudStatus `json:"status,omitempty"`
}

// XVSHNNextcloudSpec defines the desired state of a VSHNNextcloud.
type XVSHNNextcloudSpec struct {
	// Parameters are the configurable fields of a VSHNNextcloud.
	Parameters VSHNNextcloudParameters `json:"parameters,omitempty"`

	ResourceRefs []xpv1.TypedReference `json:"resourceRefs,omitempty"`

	xpv1.ResourceSpec `json:",inline"`
}

type XVSHNNextcloudStatus struct {
	VSHNNextcloudStatus `json:",inline"`
	xpv1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNNextcloudList represents a list of composites
type XVSHNNextcloudList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XVSHNNextcloud `json:"items"`
}

// GetMaintenanceDayOfWeek returns the currently set day of week
func (n *VSHNNextcloud) GetMaintenanceDayOfWeek() string {
	if n.Spec.Parameters.Maintenance.DayOfWeek != "" {
		return n.Spec.Parameters.Maintenance.DayOfWeek
	}
	return n.Status.Schedules.Maintenance.DayOfWeek
}

// GetMaintenanceTimeOfDay returns the currently set time of day
func (v *VSHNNextcloud) GetMaintenanceTimeOfDay() *TimeOfDay {
	if v.Spec.Parameters.Maintenance.TimeOfDay != "" {
		return &v.Spec.Parameters.Maintenance.TimeOfDay
	}
	return &v.Status.Schedules.Maintenance.TimeOfDay
}

// SetMaintenanceDayOfWeek sets the day of week to the given value
func (v *VSHNNextcloud) SetMaintenanceDayOfWeek(dow string) {
	v.Status.Schedules.Maintenance.DayOfWeek = dow
}

// SetMaintenanceTimeOfDay sets the time of day to the given value
func (v *VSHNNextcloud) SetMaintenanceTimeOfDay(tod TimeOfDay) {
	v.Status.Schedules.Maintenance.TimeOfDay = tod
}

// GetBackupSchedule returns the current backup schedule
func (v *VSHNNextcloud) GetBackupSchedule() string {
	if v.Spec.Parameters.Backup.Schedule != "" {
		return v.Spec.Parameters.Backup.Schedule
	}
	return v.Status.Schedules.Backup
}

// SetBackupSchedule overwrites the current backup schedule
func (v *VSHNNextcloud) SetBackupSchedule(schedule string) {
	v.Status.Schedules.Backup = schedule
}

// GetBackupRetention returns the retention definition for this backup.
func (v *VSHNNextcloud) GetBackupRetention() K8upRetentionPolicy {
	return v.Spec.Parameters.Backup.Retention
}

// GetServiceName returns the name of this service
func (v *VSHNNextcloud) GetServiceName() string {
	return "nextcloud"
}

// GetFullMaintenanceSchedule returns the maintenance schedule
func (v *VSHNNextcloud) GetFullMaintenanceSchedule() VSHNDBaaSMaintenanceScheduleSpec {
	schedule := v.Spec.Parameters.Maintenance
	schedule.DayOfWeek = v.GetMaintenanceDayOfWeek()
	schedule.TimeOfDay = *v.GetMaintenanceTimeOfDay()
	return schedule
}

// GetAllowAllNamespaces returns the AllowAllNamespaces field of this service
func (v *VSHNNextcloud) GetAllowAllNamespaces() bool {
	return v.Spec.Parameters.Security.AllowAllNamespaces
}

// GetAllowedNamespaces returns the AllowedNamespaces array of this service
func (v *VSHNNextcloud) GetAllowedNamespaces() []string {
	if v.Spec.Parameters.Security.AllowedNamespaces == nil {
		v.Spec.Parameters.Security.AllowedNamespaces = []string{}
	}
	return append(v.Spec.Parameters.Security.AllowedNamespaces, v.GetClaimNamespace())
}

func (v *VSHNNextcloud) GetVSHNMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

func (v *VSHNNextcloud) GetSize() VSHNSizeSpec {
	return v.Spec.Parameters.Size
}

func (v *VSHNNextcloud) GetMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

func (v *VSHNNextcloud) GetInstances() int {
	return v.Spec.Parameters.Instances
}

func (v *VSHNNextcloud) GetPDBLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": "nextcloud",
	}
}

func (v *VSHNNextcloud) GetSecurity() *Security {
	return &v.Spec.Parameters.Security
}

func (v *VSHNNextcloud) GetWorkloadPodTemplateLabelsManager() PodTemplateLabelsManager {
	return &DeploymentManager{}
}

// This method returns the main workload name of the VSHNExtcloud.
// In this particular case it's deployment name, generated by helm chart.
func (v *VSHNNextcloud) GetWorkloadName() string {
	if strings.Contains(v.GetName(), "nextcloud") {
		return v.GetName()
	}
	return v.GetName() + "-nextcloud"
}

func (v *VSHNNextcloud) GetBillingName() string {
	return "appcat-" + v.GetServiceName()
}

func (v *VSHNNextcloud) GetSLA() string {
	return string(v.Spec.Parameters.Service.ServiceLevel)
}
