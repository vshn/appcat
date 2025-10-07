package v1

import (
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnmariadbs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnmariadbs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnmariadbs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnmariadbs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.tls.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnmariadbs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnmariadbs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.properties.retention.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnmariadbs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.security.default={})"

// +kubebuilder:object:root=true

// VSHNMariaDB is the API for creating MariaDB instances.
type VSHNMariaDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNMariaDB.
	Spec VSHNMariaDBSpec `json:"spec,omitempty"`

	// Status reflects the observed state of a VSHNMariaDB.
	Status VSHNMariaDBStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type VSHNMariaDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNMariaDB `json:"items,omitempty"`
}

// VSHNMariaDBSpec defines the desired state of a VSHNMariaDB.
type VSHNMariaDBSpec struct {
	// Parameters are the configurable fields of a VSHNMariaDB.
	Parameters VSHNMariaDBParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

// VSHNMariaDBParameters are the configurable fields of a VSHNMariaDB.
type VSHNMariaDBParameters struct {
	// Service contains MariaDB DBaaS specific properties
	Service VSHNMariaDBServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNSizeSpec `json:"size,omitempty"`

	// Scheduling contains settings to control the scheduling of an instance.
	Scheduling VSHNDBaaSSchedulingSpec `json:"scheduling,omitempty"`

	// TLS contains settings to control tls traffic of a service.
	TLS VSHNMariaDBTLSSpec `json:"tls,omitempty"`

	// Backup contains settings to control how the instance should get backed up.
	Backup K8upBackupSpec `json:"backup,omitempty"`

	// Restore contains settings to control the restore of an instance.
	Restore K8upRestoreSpec `json:"restore,omitempty"`

	// StorageClass configures the storageClass to use for the PVC used by MariaDB.
	StorageClass string `json:"storageClass,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance VSHNDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`

	// Monitoring contains settings to control monitoring.
	Monitoring VSHNMonitoring `json:"monitoring,omitempty"`

	// Network contains any network related settings.
	Network VSHNDBaaSNetworkSpec `json:"network,omitempty"`

	// Security defines the security of a service
	Security Security `json:"security,omitempty"`

	// +kubebuilder:default=1
	// +kubebuilder:validation:Enum=1;3;

	// Instances configures the number of MariaDB instances for the cluster.
	// Each instance contains one MariaDB server.
	// These serves will form a Galera cluster.
	// An additional ProxySQL statefulset will be deployed to make failovers
	// as seamless as possible.
	Instances int `json:"instances,omitempty"`
}

// VSHNMariaDBServiceSpec contains MariaDB DBaaS specific properties
type VSHNMariaDBServiceSpec struct {
	// +kubebuilder:validation:Enum="10.4";"10.5";"10.6";"10.9";"10.10";"10.11";"11.0";"11.1";"11.2";"11.3";"11.4";"11.5";"11.8"
	// +kubebuilder:default="11.8"

	// Version contains supported version of MariaDB.
	// Multiple versions are supported. The latest version "11.8" is the default version.
	Version string `json:"version,omitempty"`

	// MariadbSettings contains additional MariaDB settings.
	MariadbSettings string `json:"mariadbSettings,omitempty"`

	// +kubebuilder:validation:Enum="besteffort";"guaranteed"
	// +kubebuilder:default="besteffort"

	// ServiceLevel defines the service level of this service. Either Best Effort or Guaranteed Availability is allowed.
	ServiceLevel VSHNDBaaSServiceLevel `json:"serviceLevel,omitempty"`

	// Access defines additional users and databases for this instance.
	Access []VSHNAccess `json:"access,omitempty"`
}

// VSHNMariaDBTLSSpec contains settings to control tls traffic of a service.
type VSHNMariaDBTLSSpec struct {
	// +kubebuilder:default=true

	// TLSEnabled enables TLS traffic for the service
	TLSEnabled bool `json:"enabled,omitempty"`

	// +kubebuilder:default=true
	// TLSAuthClients enables client authentication requirement
	TLSAuthClients bool `json:"authClients,omitempty"`
}

// VSHNMariaDBStatus reflects the observed state of a VSHNMariaDB.
type VSHNMariaDBStatus struct {
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
	// CurrentInstances tracks the current amount of instances.
	// Mainly used to detect if there was a change in instances
	CurrentInstances int `json:"currentInstances,omitempty"`
	// MariaDBVersion contains the current MariaDB server version
	MariaDBVersion string `json:"mariadbVersion,omitempty"`
	// InitialMaintenanceRan tracks if the initial maintenance job has been triggered
	InitialMaintenanceRan bool `json:"initialMaintenanceRan,omitempty"`
	// ResourceStatus represents the observed state of a managed resource.
	xpv1.ResourceStatus `json:",inline"`
}

func (v *VSHNMariaDB) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

func (v *VSHNMariaDB) GetClaimName() string {
	return v.GetLabels()["crossplane.io/claim-name"]
}

func (v *VSHNMariaDB) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-mariadb-%s", v.GetName())
}

func (v *VSHNMariaDB) SetInstanceNamespaceStatus() {
	v.Status.InstanceNamespace = v.GetInstanceNamespace()
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNMariaDB represents the internal composite of this claim
type XVSHNMariaDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XVSHNMariaDBSpec   `json:"spec"`
	Status XVSHNMariaDBStatus `json:"status,omitempty"`
}

func (v *XVSHNMariaDB) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-mariadb-%s", v.GetName())
}

// XVSHNMariaDBSpec defines the desired state of a VSHNMariaDB.
type XVSHNMariaDBSpec struct {
	// Parameters are the configurable fields of a VSHNMariaDB.
	Parameters VSHNMariaDBParameters `json:"parameters,omitempty"`

	xpv1.ResourceSpec `json:",inline"`
}

type XVSHNMariaDBStatus struct {
	VSHNMariaDBStatus   `json:",inline"`
	xpv1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNMariaDBList represents a list of composites
type XVSHNMariaDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XVSHNMariaDB `json:"items"`
}

// GetMaintenanceDayOfWeek returns the currently set day of week
func (v *VSHNMariaDB) GetMaintenanceDayOfWeek() string {
	if v.Spec.Parameters.Maintenance.DayOfWeek != "" {
		return v.Spec.Parameters.Maintenance.DayOfWeek
	}
	return v.Status.Schedules.Maintenance.DayOfWeek
}

// GetMaintenanceTimeOfDay returns the currently set time of day
func (v *VSHNMariaDB) GetMaintenanceTimeOfDay() *TimeOfDay {
	if v.Spec.Parameters.Maintenance.TimeOfDay != "" {
		return &v.Spec.Parameters.Maintenance.TimeOfDay
	}
	return &v.Status.Schedules.Maintenance.TimeOfDay
}

// SetMaintenanceDayOfWeek sets the day of week to the given value
func (v *VSHNMariaDB) SetMaintenanceDayOfWeek(dow string) {
	v.Status.Schedules.Maintenance.DayOfWeek = dow
}

// SetMaintenanceTimeOfDay sets the time of day to the given value
func (v *VSHNMariaDB) SetMaintenanceTimeOfDay(tod TimeOfDay) {
	v.Status.Schedules.Maintenance.TimeOfDay = tod
}

// GetBackupSchedule returns the current backup schedule
func (v *VSHNMariaDB) GetBackupSchedule() string {
	if v.Spec.Parameters.Backup.Schedule != "" {
		return v.Spec.Parameters.Backup.Schedule
	}
	return v.Status.Schedules.Backup
}

// SetBackupSchedule overwrites the current backup schedule
func (v *VSHNMariaDB) SetBackupSchedule(schedule string) {
	v.Status.Schedules.Backup = schedule
}

// GetBackupRetention returns the retention definition for this backup.
func (v *VSHNMariaDB) GetBackupRetention() K8upRetentionPolicy {
	return v.Spec.Parameters.Backup.Retention
}

// GetServiceName returns the name of this service
func (v *VSHNMariaDB) GetServiceName() string {
	return "mariadb"
}

// GetFullMaintenanceSchedule returns
func (v *VSHNMariaDB) GetFullMaintenanceSchedule() VSHNDBaaSMaintenanceScheduleSpec {
	schedule := v.Spec.Parameters.Maintenance
	schedule.DayOfWeek = v.GetMaintenanceDayOfWeek()
	schedule.TimeOfDay = *v.GetMaintenanceTimeOfDay()
	return schedule
}

// GetAllowAllNamespaces returns the AllowAllNamespaces field of this service
func (v *VSHNMariaDB) GetAllowAllNamespaces() bool {
	return v.Spec.Parameters.Security.AllowAllNamespaces
}

// GetAllowedNamespaces returns the AllowedNamespaces array of this service
func (v *VSHNMariaDB) GetAllowedNamespaces() []string {
	if v.Spec.Parameters.Security.AllowedNamespaces == nil {
		v.Spec.Parameters.Security.AllowedNamespaces = []string{}
	}
	return append(v.Spec.Parameters.Security.AllowedNamespaces, v.GetClaimNamespace())
}

func (v *VSHNMariaDB) GetVSHNMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

func (v *VSHNMariaDB) GetSize() VSHNSizeSpec {
	return v.Spec.Parameters.Size
}

func (v *VSHNMariaDB) GetMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

func (v *VSHNMariaDB) GetInstances() int {
	return v.Spec.Parameters.Instances
}

func (v *VSHNMariaDB) GetPDBLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name:": "mariadb-galera",
	}
}

func (v *VSHNMariaDB) GetSecurity() *Security {
	return &v.Spec.Parameters.Security
}

func (v *VSHNMariaDB) GetWorkloadPodTemplateLabelsManager() PodTemplateLabelsManager {
	return &StatefulSetManager{}
}

func (v *VSHNMariaDB) GetWorkloadName() string {
	return v.GetName()
}

func (v *VSHNMariaDB) GetBillingName() string {
	return "appcat-" + v.GetServiceName()
}

func (v *VSHNMariaDB) GetSLA() string {
	return string(v.Spec.Parameters.Service.ServiceLevel)
}

// IsBackupEnabled returns true if backups are enabled for this instance
func (v *VSHNMariaDB) IsBackupEnabled() bool {
	return v.Spec.Parameters.Backup.IsEnabled()
}
