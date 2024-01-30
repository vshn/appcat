package v1

import (
	"fmt"
	"strings"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	v1 "github.com/vshn/appcat/v4/apis/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnmariadbs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnmariadbs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnmariadbs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnmariadbs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.tls.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnmariadbs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.default={})"

// +kubebuilder:object:root=true

// VSHNMariaDB is the API for creating MariaDB instances.
type VSHNMariaDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNMariaDB.
	Spec VSHNMariaDBSpec `json:"spec"`

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
	WriteConnectionSecretToRef v1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
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
}

// VSHNMariaDBServiceSpec contains MariaDB DBaaS specific properties
type VSHNMariaDBServiceSpec struct {
	// +kubebuilder:validation:Enum="10.4";"10.5";"10.6";"10.9";"10.10";"10.11";"11.0";"11.1";"11.2";
	// +kubebuilder:default="11.2"

	// Version contains supported version of MariaDB.
	// Multiple versions are supported. The latest version "11.2" is the default version.
	Version string `json:"version,omitempty"`

	// MariadbSettings contains additional MariaDB settings.
	MariadbSettings string `json:"mariadbSettings,omitempty"`

	// +kubebuilder:validation:Enum="besteffort";"guaranteed"
	// +kubebuilder:default="besteffort"

	// ServiceLevel defines the service level of this service. Either Best Effort or Guaranteed Availability is allowed.
	ServiceLevel VSHNDBaaSServiceLevel `json:"serviceLevel,omitempty"`

	// +kubebuilder:default="standalone"
	// +kubebuilder:validation:Enum=replication;standalone
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

func (v *VSHNMariaDB) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

func (v *VSHNMariaDB) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-mariadb-%s", v.GetName())
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
func (n *VSHNMariaDB) GetMaintenanceDayOfWeek() string {
	if n.Spec.Parameters.Maintenance.DayOfWeek != "" {
		return n.Spec.Parameters.Maintenance.DayOfWeek
	}
	return n.Status.Schedules.Maintenance.DayOfWeek
}

// GetMaintenanceTimeOfDay returns the currently set time of day
func (v *VSHNMariaDB) GetMaintenanceTimeOfDay() string {
	if v.Spec.Parameters.Maintenance.TimeOfDay != "" {
		return v.Spec.Parameters.Maintenance.TimeOfDay
	}
	return v.Status.Schedules.Maintenance.TimeOfDay
}

// SetMaintenanceDayOfWeek sets the day of week to the given value
func (v *VSHNMariaDB) SetMaintenanceDayOfWeek(dow string) {
	v.Status.Schedules.Maintenance.DayOfWeek = dow
}

// SetMaintenanceTimeOfDay sets the time of day to the given value
func (v *VSHNMariaDB) SetMaintenanceTimeOfDay(tod string) {
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
	schedule.TimeOfDay = v.GetMaintenanceTimeOfDay()
	return schedule
}

// Get InstanceNamespaceRegex returns regex for prometheus rules, splitted insatnce namespace and error if necessary
func (mdb *VSHNMariaDB) GetInstanceNamespaceRegex() (string, []string, error) {
	// from vshn-postgresql-customer-namespace-whatever
	// make vshn-postgresql-(.+)-.+
	// required for Prometheus queries
	instanceNamespace := mdb.GetInstanceNamespace()
	// vshn- <- takes 5 letters, anything shorter that 7 makes no sense
	if len(instanceNamespace) < 7 {
		return "", nil, fmt.Errorf("giveMeNamespaceRegex: instance namespace is way too short")
	}

	splitted := strings.Split(instanceNamespace, "-")
	// at least [vshn, serviceName] should be present
	if len(instanceNamespace) < 2 {
		return "", nil, fmt.Errorf("giveMeNamespaceRegex: instance namespace broken during splitting")
	}

	return fmt.Sprintf("%s-%s-(.+)-.+", splitted[0], splitted[1]), splitted, nil
}
