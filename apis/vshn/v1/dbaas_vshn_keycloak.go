package v1

import (
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	v1 "github.com/vshn/appcat/v4/apis/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnkeycloaks.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnkeycloaks.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnkeycloaks.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnkeycloaks.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.properties.postgreSQLParameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnkeycloaks.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.tls.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnkeycloaks.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.default={})"

// +kubebuilder:object:root=true

// VSHNKeycloak is the API for creating keycloak instances.
type VSHNKeycloak struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNKeycloak.
	Spec VSHNKeycloakSpec `json:"spec"`

	// Status reflects the observed state of a VSHNKeycloak.
	Status VSHNKeycloakStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type VSHNKeycloakList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNKeycloak `json:"items,omitempty"`
}

// VSHNKeycloakSpec defines the desired state of a VSHNKeycloak.
type VSHNKeycloakSpec struct {
	// Parameters are the configurable fields of a VSHNKeycloak.
	Parameters VSHNKeycloakParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef v1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

// VSHNKeycloakParameters are the configurable fields of a VSHNKeycloak.
type VSHNKeycloakParameters struct {
	// Service contains keycloak DBaaS specific properties
	Service VSHNKeycloakServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNSizeSpec `json:"size,omitempty"`

	// Scheduling contains settings to control the scheduling of an instance.
	Scheduling VSHNDBaaSSchedulingSpec `json:"scheduling,omitempty"`

	// TLS contains settings to control tls traffic of a service.
	TLS VSHNKeycloakTLSSpec `json:"tls,omitempty"`

	// Backup contains settings to control how the instance should get backed up.
	Backup K8upBackupSpec `json:"backup,omitempty"`

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

	// Instances configures the number of Keycloak instances for the cluster.
	// Each instance contains one Keycloak server.
	Instances int `json:"instances,omitempty"`
}

// VSHNKeycloakServiceSpec contains keycloak DBaaS specific properties
type VSHNKeycloakServiceSpec struct {
	// FQDN contains the FQDN which will be used for the ingress.
	// If it's not set, no ingress will be deployed.
	// This also enables strict hostname checking for this FQDN.
	FQDN string `json:"fqdn,omitempty"`

	// RelativePath on which Keycloak will listen.
	// +kubebuilder:default="/"
	RelativePath string `json:"relativePath,omitempty"`

	// +kubebuilder:validation:Enum="23";"24"
	// +kubebuilder:default="23"

	// Version contains supported version of keycloak.
	// Multiple versions are supported. The latest version 23 is the default version.
	Version string `json:"version,omitempty"`

	// +kubebuilder:validation:Enum="besteffort";"guaranteed"
	// +kubebuilder:default="besteffort"

	// ServiceLevel defines the service level of this service. Either Best Effort or Guaranteed Availability is allowed.
	ServiceLevel VSHNDBaaSServiceLevel `json:"serviceLevel,omitempty"`

	// PostgreSQLParameters can be used to set any supported setting in the
	// underlying PostgreSQL instance.
	PostgreSQLParameters *VSHNPostgreSQLParameters `json:"postgreSQLParameters,omitempty"`

	// CustomizationImage can be used to provide an image with custom themes and providers.
	// The themes need to be be placed in the `/themes` directory of the custom image.
	// the providers need to be placed in the `/providers` directory of the custom image.
	CustomizationImage VSHNKeycloakCustomizationImage `json:"customizationImage,omitempty"`

	// CustomConfigurationRef can be used to provide a configmap containing configurations for the
	// keycloak instance. The config is a JSON file based on the keycloak export files.
	// The referenced configmap, must have the configuration in a field called `keycloak-config.json`
	CustomConfigurationRef *string `json:"customConfigurationRef,omitempty"`

	// CustomEnvVariablesRef can be used to provide custom environment variables from a
	// provided secret for the keycloak instance. The environment variables provided
	// can for example be used in the custom JSON configuration provided in the `Configuration`
	// field with `$(env:<ENV_VAR_NAME>:-<some_default_value>)`
	CustomEnvVariablesRef *string `json:"customEnvVariablesRef,omitempty"`
}

type VSHNKeycloakCustomizationImage struct {
	// Path to a valid image
	Image string `json:"image,omitempty"`

	// Reference to an imagePullSecret
	ImagePullSecretRef corev1.SecretReference `json:"imagePullSecretRef,omitempty"`
}

// VSHNKeycloakSettings contains Keycloak specific settings.
type VSHNKeycloakSettings struct{}

// VSHNKeycloakSizeSpec contains settings to control the sizing of a service.
type VSHNKeycloakSizeSpec struct {

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

// VSHNKeycloakTLSSpec contains settings to control tls traffic of a service.
type VSHNKeycloakTLSSpec struct {
	// +kubebuilder:default=true

	// TLSEnabled enables TLS traffic for the service
	TLSEnabled bool `json:"enabled,omitempty"`

	// +kubebuilder:default=true
	// TLSAuthClients enables client authentication requirement
	TLSAuthClients bool `json:"authClients,omitempty"`
}

// VSHNKeycloakStatus reflects the observed state of a VSHNKeycloak.
type VSHNKeycloakStatus struct {
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
	// Schedules keeps track of random generated schedules, is overwriten by
	// schedules set in the service's spec.
	Schedules VSHNScheduleStatus `json:"schedules,omitempty"`
}

func (v *VSHNKeycloak) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

func (v *VSHNKeycloak) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-keycloak-%s", v.GetName())
}

func (v *XVSHNKeycloak) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-keycloak-%s", v.GetName())
}

func (v *VSHNKeycloak) SetInstanceNamespaceStatus() {
	v.Status.InstanceNamespace = v.GetInstanceNamespace()
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNKeycloak represents the internal composite of this claim
type XVSHNKeycloak struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XVSHNKeycloakSpec   `json:"spec"`
	Status XVSHNKeycloakStatus `json:"status,omitempty"`
}

// XVSHNKeycloakSpec defines the desired state of a VSHNKeycloak.
type XVSHNKeycloakSpec struct {
	// Parameters are the configurable fields of a VSHNKeycloak.
	Parameters VSHNKeycloakParameters `json:"parameters,omitempty"`

	xpv1.ResourceSpec `json:",inline"`
}

type XVSHNKeycloakStatus struct {
	VSHNKeycloakStatus  `json:",inline"`
	xpv1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNKeycloakList represents a list of composites
type XVSHNKeycloakList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XVSHNKeycloak `json:"items"`
}

// GetMaintenanceDayOfWeek returns the currently set day of week
func (n *VSHNKeycloak) GetMaintenanceDayOfWeek() string {
	if n.Spec.Parameters.Maintenance.DayOfWeek != "" {
		return n.Spec.Parameters.Maintenance.DayOfWeek
	}
	return n.Status.Schedules.Maintenance.DayOfWeek
}

// GetMaintenanceTimeOfDay returns the currently set time of day
func (v *VSHNKeycloak) GetMaintenanceTimeOfDay() *TimeOfDay {
	if v.Spec.Parameters.Maintenance.TimeOfDay != "" {
		return &v.Spec.Parameters.Maintenance.TimeOfDay
	}
	return &v.Status.Schedules.Maintenance.TimeOfDay
}

// SetMaintenanceDayOfWeek sets the day of week to the given value
func (v *VSHNKeycloak) SetMaintenanceDayOfWeek(dow string) {
	v.Status.Schedules.Maintenance.DayOfWeek = dow
}

// SetMaintenanceTimeOfDay sets the time of day to the given value
func (v *VSHNKeycloak) SetMaintenanceTimeOfDay(tod TimeOfDay) {
	v.Status.Schedules.Maintenance.TimeOfDay = tod
}

// GetBackupSchedule returns the current backup schedule
func (v *VSHNKeycloak) GetBackupSchedule() string {
	if v.Spec.Parameters.Backup.Schedule != "" {
		return v.Spec.Parameters.Backup.Schedule
	}
	return v.Status.Schedules.Backup
}

// SetBackupSchedule overwrites the current backup schedule
func (v *VSHNKeycloak) SetBackupSchedule(schedule string) {
	v.Status.Schedules.Backup = schedule
}

// GetBackupRetention returns the retention definition for this backup.
func (v *VSHNKeycloak) GetBackupRetention() K8upRetentionPolicy {
	return v.Spec.Parameters.Backup.Retention
}

// GetServiceName returns the name of this service
func (v *VSHNKeycloak) GetServiceName() string {
	return "keycloak"
}

// GetFullMaintenanceSchedule returns the maintenance schedule
func (v *VSHNKeycloak) GetFullMaintenanceSchedule() VSHNDBaaSMaintenanceScheduleSpec {
	schedule := v.Spec.Parameters.Maintenance
	schedule.DayOfWeek = v.GetMaintenanceDayOfWeek()
	schedule.TimeOfDay = *v.GetMaintenanceTimeOfDay()
	return schedule
}

// GetAllowAllNamespaces returns the AllowAllNamespaces field of this service
func (v *VSHNKeycloak) GetAllowAllNamespaces() bool {
	return v.Spec.Parameters.Security.AllowAllNamespaces
}

// GetAllowedNamespaces returns the AllowedNamespaces array of this service
func (v *VSHNKeycloak) GetAllowedNamespaces() []string {
	if v.Spec.Parameters.Security.AllowedNamespaces == nil {
		v.Spec.Parameters.Security.AllowedNamespaces = []string{}
	}
	return append(v.Spec.Parameters.Security.AllowedNamespaces, v.GetClaimNamespace())
}

func (v *VSHNKeycloak) GetVSHNMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

func (v *VSHNKeycloak) GetSize() VSHNSizeSpec {
	return v.Spec.Parameters.Size
}

func (v *VSHNKeycloak) GetMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

func (v *VSHNKeycloak) GetInstances() int {
	return v.Spec.Parameters.Instances
}

func (v *VSHNKeycloak) GetPDBLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": "keycloakx",
	}
}
