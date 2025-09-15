package v1

import (
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnpostgresqlcnpgs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnpostgresqlcnpgs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnpostgresqlcnpgs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnpostgresqlcnpgs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.properties.tls.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnpostgresqlcnpgs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnpostgresqlcnpgs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.maintenance.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnpostgresqlcnpgs.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.security.default={})"

// +kubebuilder:object:root=true

// VSHNPostgreSQLCNPG is the API for creating Postgresql clusters.
type VSHNPostgreSQLCNPG struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNPostgreSQLCNPG.
	Spec VSHNPostgreSQLCNPGSpec `json:"spec"`

	// Status reflects the observed state of a VSHNPostgreSQLCNPG.
	Status VSHNPostgreSQLCNPGStatus `json:"status,omitempty"`
}

// VSHNPostgreSQLCNPGSpec defines the desired state of a VSHNPostgreSQLCNPG.
type VSHNPostgreSQLCNPGSpec struct {
	// Parameters are the configurable fields of a VSHNPostgreSQLCNPG.
	Parameters        VSHNPostgreSQLCNPGParameters `json:"parameters,omitempty"`
	xpv1.ResourceSpec `json:",inline"`
}

// VSHNPostgreSQLCNPGParameters are the configurable fields of a VSHNPostgreSQLCNPG.
type VSHNPostgreSQLCNPGParameters struct {
	// Service contains PostgreSQL DBaaS specific properties
	Service VSHNPostgreSQLCNPGServiceSpec `json:"service,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance VSHNDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNSizeSpec `json:"size,omitempty"`

	// Scheduling contains settings to control the scheduling of an instance.
	Scheduling VSHNDBaaSSchedulingSpec `json:"scheduling,omitempty"`

	// Network contains any network related settings.
	Network VSHNDBaaSNetworkSpec `json:"network,omitempty"`

	// Backup contains settings to control the backups of an instance.
	Backup VSHNPostgreSQLCNPGBackup `json:"backup,omitempty"`

	// Restore contains settings to control the restore of an instance.
	Restore *VSHNPostgreSQLCNPGRestore `json:"restore,omitempty"`

	// Monitoring contains settings to control monitoring.
	Monitoring VSHNMonitoring `json:"monitoring,omitempty"`

	// Encryption contains settings to control the storage encryption of an instance.
	Encryption VSHNPostgreSQLCNPGEncryption `json:"encryption,omitempty"`

	// UpdateStrategy indicates when updates to the instance spec will be applied.
	UpdateStrategy VSHNPostgreSQLCNPGUpdateStrategy `json:"updateStrategy,omitempty"`

	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3

	// Instances configures the number of PostgreSQL instances for the cluster.
	// Each instance contains one Postgres server.
	// Out of all Postgres servers, one is elected as the primary, the rest remain as read-only replicas.
	Instances int `json:"instances,omitempty"`

	// Security defines the security of a service
	Security Security `json:"security,omitempty"`
}

const VSHNPostgreSQLCNPGUpdateStrategyTypeImmediate = "Immediate"
const VSHNPostgreSQLCNPGUpdateStrategyTypeOnRestart = "OnRestart"

// VSHNPostgreSQLCNPGUpdateStrategy indicates how and when updates to the instance spec will be applied.
type VSHNPostgreSQLCNPGUpdateStrategy struct {
	// +kubebuilder:validation:Enum="Immediate";"OnRestart"
	// +kubebuilder:default="Immediate"

	// Type indicates the type of the UpdateStrategy. Default is Immediate.
	// Possible enum values:
	//   - `"OnRestart"` indicates that the changes to the spec will only be applied once the instance is restarted by other means, most likely during maintenance.
	//   - `"Immediate"` indicates that update will be applied to the instance as soon as the spec changes. Please be aware that this might lead to short downtime.
	Type string `json:"type,omitempty"`
}

// VSHNPostgreSQLCNPGServiceSpec contains PostgreSQL DBaaS specific properties
type VSHNPostgreSQLCNPGServiceSpec struct {
	// +kubebuilder:validation:Enum="12";"13";"14";"15";"16";"17"
	// +kubebuilder:default="15"

	// MajorVersion contains supported version of PostgreSQL.
	// Multiple versions are supported. The latest version "15" is the default version.
	MajorVersion string `json:"majorVersion,omitempty"`

	// PGSettings contains additional PostgreSQL settings.
	PostgreSQLSettings runtime.RawExtension `json:"pgSettings,omitempty"`

	// +kubebuilder:validation:Enum="besteffort";"guaranteed"
	// +kubebuilder:default="besteffort"

	// ServiceLevel defines the service level of this service. Either Best Effort or Guaranteed Availability is allowed.
	ServiceLevel VSHNDBaaSServiceLevel `json:"serviceLevel,omitempty"`
}

type VSHNPostgreSQLCNPGBackup struct {
	// +kubebuilder:validation:Pattern=^(\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([0-9]|1[0-9]|2[0-3])|\*\/([0-9]|1[0-9]|2[0-3])) (\*|([1-9]|1[0-9]|2[0-9]|3[0-1])|\*\/([1-9]|1[0-9]|2[0-9]|3[0-1])) (\*|([1-9]|1[0-2])|\*\/([1-9]|1[0-2])) (\*|([0-6])|\*\/([0-6]))$
	Schedule string `json:"schedule,omitempty"`

	// +kubebuilder:validation:Pattern="^[1-9][0-9]*$"
	// +kubebuilder:default=6
	// +kubebuilder:validation:XIntOrString
	Retention int `json:"retention,omitempty"`

	// DeletionProtection will protect the instance from being deleted for the given retention time.
	// This is enabled by default.
	// +kubebuilder:default=true
	DeletionProtection *bool `json:"deletionProtection,omitempty"`

	// DeletionRetention specifies in days how long the instance should be kept after deletion.
	// The default is keeping it one week.
	// +kubebuilder:default=7
	DeletionRetention int `json:"deletionRetention,omitempty"`
}

// GetBackupSchedule gets the currently set schedule
func (v *VSHNPostgreSQLCNPGBackup) GetBackupSchedule() string {
	return v.Schedule
}

// SetBackupSchedule sets the schedule to the given value
func (v *VSHNPostgreSQLCNPGBackup) SetBackupSchedule(schedule string) {
	v.Schedule = schedule
}

// VSHNPostgreSQLCNPGRestore contains restore specific parameters.
type VSHNPostgreSQLCNPGRestore struct {

	// ClaimName specifies the name of the instance you want to restore from.
	// The claim has to be in the same namespace as this new instance.
	ClaimName string `json:"claimName,omitempty"`

	// ClaimType specifies the type of the instance you want to restore from.
	ClaimType string `json:"claimType,omitempty"`

	// BackupName is the name of the specific backup you want to restore.
	BackupName string `json:"backupName,omitempty"`

	// RecoveryTimeStamp an ISO 8601 date, that holds UTC date indicating at which point-in-time the database has to be restored.
	// This is optional and if no PIT recovery is required, it can be left empty.
	// +kubebuilder:validation:Pattern=`^(?:[1-9]\d{3}-(?:(?:0[1-9]|1[0-2])-(?:0[1-9]|1\d|2[0-8])|(?:0[13-9]|1[0-2])-(?:29|30)|(?:0[13578]|1[02])-31)|(?:[1-9]\d(?:0[48]|[2468][048]|[13579][26])|(?:[2468][048]|[13579][26])00)-02-29)T(?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d(?:Z|[+-][01]\d:[0-5]\d)$`
	RecoveryTimeStamp string `json:"recoveryTimeStamp,omitempty"`
}

func (v *VSHNPostgreSQLCNPG) GetVSHNMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

// VSHNPostgreSQLCNPGEncryption contains storage encryption specific parameters
type VSHNPostgreSQLCNPGEncryption struct {

	// Enabled specifies if the instance should use encrypted storage for the instance.
	Enabled bool `json:"enabled,omitempty"`
}

// VSHNPostgreSQLCNPGTLS contains TLS specific parameters
type VSHNPostgreSQLCNPGTLS struct {
	// Enabled specifies if the instance should use TLS for the instance.
	// This change takes effect immediately and does not require a restart of the database.
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`
}

// VSHNPostgreSQLCNPGStatus reflects the observed state of a VSHNPostgreSQLCNPG.
type VSHNPostgreSQLCNPGStatus struct {
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`

	// CurrentVersion contains the current version of PostgreSQL.
	CurrentVersion string `json:"currentVersion,omitempty"`

	// PreviousVersion contains the previous version of PostgreSQL.
	PreviousVersion string `json:"previousVersion,omitempty"`

	// PostgreSQLConditions contains the status conditions of the backing object.
	PostgreSQLConditions         []Condition `json:"postgresqlConditions,omitempty"`
	NamespaceConditions          []Condition `json:"namespaceConditions,omitempty"`
	ProfileConditions            []Condition `json:"profileConditions,omitempty"`
	PGConfigConditions           []Condition `json:"pgconfigConditions,omitempty"`
	PGClusterConditions          []Condition `json:"pgclusterConditions,omitempty"`
	SecretsConditions            []Condition `json:"secretConditions,omitempty"`
	ObjectBucketConditions       []Condition `json:"objectBucketConditions,omitempty"`
	ObjectBackupConfigConditions []Condition `json:"objectBackupConfigConditions,omitempty"`
	NetworkPolicyConditions      []Condition `json:"networkPolicyConditions,omitempty"`
	LocalCAConditions            []Condition `json:"localCAConditions,omitempty"`
	CertificateConditions        []Condition `json:"certificateConditions,omitempty"`

	// IsEOL indicates if this instance is using an EOL version of PostgreSQL.
	IsEOL bool `json:"isEOL,omitempty"`
	// Schedules keeps track of random generated schedules, is overwriten by
	// schedules set in the service's spec.
	Schedules VSHNScheduleStatus `json:"schedules,omitempty"`
	// ResourceStatus represents the observed state of a managed resource.
	xpv1.ResourceStatus `json:",inline"`
}

func (v *VSHNPostgreSQLCNPG) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

func (v *VSHNPostgreSQLCNPG) GetClaimName() string {
	return v.GetLabels()["crossplane.io/claim-name"]
}

// +kubebuilder:object:root=true

// VSHNPostgreSQLCNPGList defines a list of VSHNPostgreSQLCNPG
type VSHNPostgreSQLCNPGList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNPostgreSQLCNPG `json:"items"`
}

// GetMaintenanceDayOfWeek returns the currently set day of week
func (v *VSHNPostgreSQLCNPG) GetMaintenanceDayOfWeek() string {
	if v.Spec.Parameters.Maintenance.DayOfWeek != "" {
		return v.Spec.Parameters.Maintenance.DayOfWeek
	}
	return v.Status.Schedules.Maintenance.DayOfWeek
}

// GetMaintenanceTimeOfDay returns the currently set time of day
func (v *VSHNPostgreSQLCNPG) GetMaintenanceTimeOfDay() *TimeOfDay {
	if v.Spec.Parameters.Maintenance.TimeOfDay != "" {
		return &v.Spec.Parameters.Maintenance.TimeOfDay
	}
	return &v.Status.Schedules.Maintenance.TimeOfDay
}

// SetMaintenanceDayOfWeek sets the day of week to the given value
func (v *VSHNPostgreSQLCNPG) SetMaintenanceDayOfWeek(dow string) {
	v.Status.Schedules.Maintenance.DayOfWeek = dow
}

// SetMaintenanceTimeOfDay sets the time of day to the given value
func (v *VSHNPostgreSQLCNPG) SetMaintenanceTimeOfDay(tod TimeOfDay) {
	v.Status.Schedules.Maintenance.TimeOfDay = tod
}

// GetBackupSchedule returns the current backup schedule
func (v *VSHNPostgreSQLCNPG) GetBackupSchedule() string {
	if v.Spec.Parameters.Backup.Schedule != "" {
		return v.Spec.Parameters.Backup.Schedule
	}
	return v.Status.Schedules.Backup
}

// SetBackupSchedule overwrites the current backup schedule
func (v *VSHNPostgreSQLCNPG) SetBackupSchedule(schedule string) {
	v.Status.Schedules.Backup = schedule
}

// GetFullMaintenanceSchedule returns
func (v *VSHNPostgreSQLCNPG) GetFullMaintenanceSchedule() VSHNDBaaSMaintenanceScheduleSpec {
	schedule := v.Spec.Parameters.Maintenance
	schedule.DayOfWeek = v.GetMaintenanceDayOfWeek()
	schedule.TimeOfDay = *v.GetMaintenanceTimeOfDay()
	return schedule
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNPostgreSQLCNPG represents the internal composite of this claim
type XVSHNPostgreSQLCNPG struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XVSHNPostgreSQLCNPGSpec   `json:"spec"`
	Status XVSHNPostgreSQLCNPGStatus `json:"status,omitempty"`
}

type XVSHNPostgreSQLCNPGStatus struct {
	VSHNPostgreSQLCNPGStatus `json:",inline"`
	xpv1.ResourceStatus      `json:",inline"`
}

type XVSHNPostgreSQLCNPGSpec struct {
	// Parameters are the configurable fields of a VSHNPostgreSQLCNPG.
	Parameters        VSHNPostgreSQLCNPGParameters `json:"parameters,omitempty"`
	xpv1.ResourceSpec `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNPostgreSQLCNPGList represents a list of composites
type XVSHNPostgreSQLCNPGList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XVSHNPostgreSQLCNPG `json:"items"`
}

func (v *VSHNPostgreSQLCNPG) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-postgresql-%s", v.GetName())
}

func (v *VSHNPostgreSQLCNPG) SetInstanceNamespaceStatus() {
	v.Status.InstanceNamespace = v.GetInstanceNamespace()
}

func (v *XVSHNPostgreSQLCNPG) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-postgresql-%s", v.GetName())
}

// GetBackupRetention returns the retention definition for this backup.
func (v *VSHNPostgreSQLCNPG) GetBackupRetention() K8upRetentionPolicy {
	return K8upRetentionPolicy{}
}

// GetServiceName returns the name of this service
func (v *VSHNPostgreSQLCNPG) GetServiceName() string {
	return "postgresql"
}

// GetAllowAllNamespaces returns the AllowAllNamespaces field of this service
func (v *VSHNPostgreSQLCNPG) GetAllowAllNamespaces() bool {
	return v.Spec.Parameters.Security.AllowAllNamespaces
}

// GetAllowedNamespaces returns the AllowedNamespaces array of this service
func (v *VSHNPostgreSQLCNPG) GetAllowedNamespaces() []string {
	if v.Spec.Parameters.Security.AllowedNamespaces == nil {
		v.Spec.Parameters.Security.AllowedNamespaces = []string{}
	}
	return append(v.Spec.Parameters.Security.AllowedNamespaces, v.GetClaimNamespace())
}

func (v *VSHNPostgreSQLCNPG) GetSize() VSHNSizeSpec {
	return v.Spec.Parameters.Size
}

func (v *VSHNPostgreSQLCNPG) GetMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

func (v *VSHNPostgreSQLCNPG) GetInstances() int {
	return v.Spec.Parameters.Instances
}

func (v *VSHNPostgreSQLCNPG) GetPDBLabels() map[string]string {
	return map[string]string{
		"stackgres.io/cluster": "true",
	}
}

func (v *VSHNPostgreSQLCNPG) GetSecurity() *Security {
	return &v.Spec.Parameters.Security
}

func (v *VSHNPostgreSQLCNPG) GetWorkloadPodTemplateLabelsManager() PodTemplateLabelsManager {
	return &StatefulSetManager{}
}

func (v *VSHNPostgreSQLCNPG) GetWorkloadName() string {
	return v.GetName()
}

func (v *VSHNPostgreSQLCNPG) GetBillingName() string {
	return "appcat-" + v.GetServiceName()
}

func (v *VSHNPostgreSQLCNPG) GetSLA() string {
	return string(v.Spec.Parameters.Service.ServiceLevel)
}
