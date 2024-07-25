package v1

import (
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	sgv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	v1 "github.com/vshn/appcat/v4/apis/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnpostgresqls.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnpostgresqls.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnpostgresqls.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnpostgresqls.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnpostgresqls.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.maintenance.default={})"

// +kubebuilder:object:root=true

// VSHNPostgreSQL is the API for creating Postgresql clusters.
type VSHNPostgreSQL struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNPostgreSQL.
	Spec VSHNPostgreSQLSpec `json:"spec"`

	// Status reflects the observed state of a VSHNPostgreSQL.
	Status VSHNPostgreSQLStatus `json:"status,omitempty"`
}

// VSHNPostgreSQLSpec defines the desired state of a VSHNPostgreSQL.
type VSHNPostgreSQLSpec struct {
	// Parameters are the configurable fields of a VSHNPostgreSQL.
	Parameters        VSHNPostgreSQLParameters `json:"parameters,omitempty"`
	xpv1.ResourceSpec `json:",inline"`
}

// VSHNPostgreSQLParameters are the configurable fields of a VSHNPostgreSQL.
type VSHNPostgreSQLParameters struct {
	// Service contains PostgreSQL DBaaS specific properties
	Service VSHNPostgreSQLServiceSpec `json:"service,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance VSHNDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNSizeSpec `json:"size,omitempty"`

	// Scheduling contains settings to control the scheduling of an instance.
	Scheduling VSHNDBaaSSchedulingSpec `json:"scheduling,omitempty"`

	// Network contains any network related settings.
	Network VSHNDBaaSNetworkSpec `json:"network,omitempty"`

	// Backup contains settings to control the backups of an instance.
	Backup VSHNPostgreSQLBackup `json:"backup,omitempty"`

	// Restore contains settings to control the restore of an instance.
	Restore *VSHNPostgreSQLRestore `json:"restore,omitempty"`

	// Monitoring contains settings to control monitoring.
	Monitoring VSHNMonitoring `json:"monitoring,omitempty"`

	// Encryption contains settings to control the storage encryption of an instance.
	Encryption VSHNPostgreSQLEncryption `json:"encryption,omitempty"`

	// UpdateStrategy indicates when updates to the instance spec will be applied.
	UpdateStrategy VSHNPostgreSQLUpdateStrategy `json:"updateStrategy,omitempty"`

	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3

	// Instances configures the number of PostgreSQL instances for the cluster.
	// Each instance contains one Postgres server.
	// Out of all Postgres servers, one is elected as the primary, the rest remain as read-only replicas.
	Instances int `json:"instances,omitempty"`

	// This section allows to configure Postgres replication mode and HA roles groups.
	//
	// The main replication group is implicit and contains the total number of instances less the sum of all instances in other replication groups.
	Replication VSHNPostgreSQLReplicationStrategy `json:"replication,omitempty"`

	// Security defines the security of a service
	Security Security `json:"security,omitempty"`
}

type VSHNPostgreSQLReplicationStrategy struct {
	// +kubebuilder:validation:Enum="async";"sync";"strict-sync"

	// Mode defines the replication mode applied to the whole cluster. Possible values are: "async"(default), "sync", and "strict-sync"
	//
	// "async": When in asynchronous mode the cluster is allowed to lose some committed transactions.
	// When the primary server fails or becomes unavailable for any other reason a sufficiently healthy standby will automatically be promoted to primary.
	// Any transactions that have not been replicated to that standby remain in a “forked timeline” on the primary, and are effectively unrecoverable
	//
	// "sync": When in synchronous mode a standby will not be promoted unless it is certain that the standby contains all transactions that may have returned a successful commit status to client.
	//  This means that the system may be unavailable for writes even though some servers are available.
	//
	// "strict-sync": When it is absolutely necessary to guarantee that each write is stored durably on at least two nodes, use the strict synchronous mode.
	// This mode prevents synchronous replication to be switched off on the primary when no synchronous standby candidates are available.
	// As a downside, the primary will not be available for writes, blocking all client write requests until at least one synchronous replica comes up.
	//
	// NOTE: We recommend to always use three intances when setting the mode to "strict-sync".
	Mode string `json:"mode,omitempty"`
}

const VSHNPostgreSQLUpdateStrategyTypeImmediate = "Immediate"
const VSHNPostgreSQLUpdateStrategyTypeOnRestart = "OnRestart"

// VSHNPostgreSQLUpdateStrategy indicates how and when updates to the instance spec will be applied.
type VSHNPostgreSQLUpdateStrategy struct {
	// +kubebuilder:validation:Enum="Immediate";"OnRestart"
	// +kubebuilder:default="Immediate"

	// Type indicates the type of the UpdateStrategy. Default is OnRestart.
	// Possible enum values:
	//   - `"OnRestart"` indicates that the changes to the spec will only be applied once the instance is restarted by other means, most likely during maintenance.
	//   - `"Immediate"` indicates that update will be applied to the instance as soon as the spec changes. Please be aware that this might lead to short downtime.
	Type string `json:"type,omitempty"`
}

// VSHNPostgreSQLServiceSpec contains PostgreSQL DBaaS specific properties
type VSHNPostgreSQLServiceSpec struct {
	// +kubebuilder:validation:Enum="12";"13";"14";"15"
	// +kubebuilder:default="15"

	// MajorVersion contains supported version of PostgreSQL.
	// Multiple versions are supported. The latest version "15" is the default version.
	MajorVersion string `json:"majorVersion,omitempty"`

	// PGSettings contains additional PostgreSQL settings.
	PostgreSQLSettings runtime.RawExtension `json:"pgSettings,omitempty"`

	// Extensions allow to enable/disable any of the supported
	Extensions []VSHNDBaaSPostgresExtension `json:"extensions,omitempty"`

	// +kubebuilder:validation:Enum="besteffort";"guaranteed"
	// +kubebuilder:default="besteffort"

	// ServiceLevel defines the service level of this service. Either Best Effort or Guaranteed Availability is allowed.
	ServiceLevel VSHNDBaaSServiceLevel `json:"serviceLevel,omitempty"`

	// PgBouncerSettings passes additional configuration to the pgBouncer instance.
	PgBouncerSettings *sgv1.SGPoolingConfigSpecPgBouncerPgbouncerIni `json:"pgBouncerSettings,omitempty"`

	// +kubebuilder:default=true
	// This is default option if neither repack or vacuum are selected
	RepackEnabled bool `json:"repackEnabled,omitempty"`
	// +kubebuilder:default=false
	VacuumEnabled bool `json:"vacuumEnabled,omitempty"`

	Access []VSHNAccess `json:"access,omitempty"`
}

// VSHNDBaaSPostgresExtension contains the name of a single extension.
type VSHNDBaaSPostgresExtension struct {
	// Name is the name of the extension to enable.
	// For an extensive list, please consult https://stackgres.io/doc/latest/intro/extensions/
	Name string `json:"name,omitempty"`
}

type VSHNPostgreSQLBackup struct {
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
func (v *VSHNPostgreSQLBackup) GetBackupSchedule() string {
	return v.Schedule
}

// SetBackupSchedule sets the schedule to the given value
func (v *VSHNPostgreSQLBackup) SetBackupSchedule(schedule string) {
	v.Schedule = schedule
}

// VSHNPostgreSQLRestore contains restore specific parameters.
type VSHNPostgreSQLRestore struct {

	// ClaimName specifies the name of the instance you want to restore from.
	// The claim has to be in the same namespace as this new instance.
	ClaimName string `json:"claimName,omitempty"`

	// BackupName is the name of the specific backup you want to restore.
	BackupName string `json:"backupName,omitempty"`

	// RecoveryTimeStamp an ISO 8601 date, that holds UTC date indicating at which point-in-time the database has to be restored.
	// This is optional and if no PIT recovery is required, it can be left empty.
	// +kubebuilder:validation:Pattern=`^(?:[1-9]\d{3}-(?:(?:0[1-9]|1[0-2])-(?:0[1-9]|1\d|2[0-8])|(?:0[13-9]|1[0-2])-(?:29|30)|(?:0[13578]|1[02])-31)|(?:[1-9]\d(?:0[48]|[2468][048]|[13579][26])|(?:[2468][048]|[13579][26])00)-02-29)T(?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d(?:Z|[+-][01]\d:[0-5]\d)$`
	RecoveryTimeStamp string `json:"recoveryTimeStamp,omitempty"`
}

func (v *VSHNPostgreSQL) GetVSHNMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

// VSHNPostgreSQLEncryption contains storage encryption specific parameters
type VSHNPostgreSQLEncryption struct {

	// Enabled specifies if the instance should use encrypted storage for the instance.
	Enabled bool `json:"enabled,omitempty"`
}

// VSHNPostgreSQLStatus reflects the observed state of a VSHNPostgreSQL.
type VSHNPostgreSQLStatus struct {
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
	// PostgreSQLConditions contains the status conditions of the backing object.
	PostgreSQLConditions         []v1.Condition `json:"postgresqlConditions,omitempty"`
	NamespaceConditions          []v1.Condition `json:"namespaceConditions,omitempty"`
	ProfileConditions            []v1.Condition `json:"profileConditions,omitempty"`
	PGConfigConditions           []v1.Condition `json:"pgconfigConditions,omitempty"`
	PGClusterConditions          []v1.Condition `json:"pgclusterConditions,omitempty"`
	SecretsConditions            []v1.Condition `json:"secretConditions,omitempty"`
	ObjectBucketConditions       []v1.Condition `json:"objectBucketConditions,omitempty"`
	ObjectBackupConfigConditions []v1.Condition `json:"objectBackupConfigConditions,omitempty"`
	NetworkPolicyConditions      []v1.Condition `json:"networkPolicyConditions,omitempty"`
	LocalCAConditions            []v1.Condition `json:"localCAConditions,omitempty"`
	CertificateConditions        []v1.Condition `json:"certificateConditions,omitempty"`

	// IsEOL indicates if this instance is using an EOL version of PostgreSQL.
	IsEOL bool `json:"isEOL,omitempty"`
	// Schedules keeps track of random generated schedules, is overwriten by
	// schedules set in the service's spec.
	Schedules VSHNScheduleStatus `json:"schedules,omitempty"`
}

func (v *VSHNPostgreSQL) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

// +kubebuilder:object:root=true

// VSHNPostgreSQLList defines a list of VSHNPostgreSQL
type VSHNPostgreSQLList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNPostgreSQL `json:"items"`
}

// GetMaintenanceDayOfWeek returns the currently set day of week
func (v *VSHNPostgreSQL) GetMaintenanceDayOfWeek() string {
	if v.Spec.Parameters.Maintenance.DayOfWeek != "" {
		return v.Spec.Parameters.Maintenance.DayOfWeek
	}
	return v.Status.Schedules.Maintenance.DayOfWeek
}

// GetMaintenanceTimeOfDay returns the currently set time of day
func (v *VSHNPostgreSQL) GetMaintenanceTimeOfDay() *TimeOfDay {
	if v.Spec.Parameters.Maintenance.TimeOfDay != "" {
		return &v.Spec.Parameters.Maintenance.TimeOfDay
	}
	return &v.Status.Schedules.Maintenance.TimeOfDay
}

// SetMaintenanceDayOfWeek sets the day of week to the given value
func (v *VSHNPostgreSQL) SetMaintenanceDayOfWeek(dow string) {
	v.Status.Schedules.Maintenance.DayOfWeek = dow
}

// SetMaintenanceTimeOfDay sets the time of day to the given value
func (v *VSHNPostgreSQL) SetMaintenanceTimeOfDay(tod TimeOfDay) {
	v.Status.Schedules.Maintenance.TimeOfDay = tod
}

// GetBackupSchedule returns the current backup schedule
func (v *VSHNPostgreSQL) GetBackupSchedule() string {
	if v.Spec.Parameters.Backup.Schedule != "" {
		return v.Spec.Parameters.Backup.Schedule
	}
	return v.Status.Schedules.Backup
}

// SetBackupSchedule overwrites the current backup schedule
func (v *VSHNPostgreSQL) SetBackupSchedule(schedule string) {
	v.Status.Schedules.Backup = schedule
}

// GetFullMaintenanceSchedule returns
func (v *VSHNPostgreSQL) GetFullMaintenanceSchedule() VSHNDBaaSMaintenanceScheduleSpec {
	schedule := v.Spec.Parameters.Maintenance
	schedule.DayOfWeek = v.GetMaintenanceDayOfWeek()
	schedule.TimeOfDay = *v.GetMaintenanceTimeOfDay()
	return schedule
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNPostgreSQL represents the internal composite of this claim
type XVSHNPostgreSQL struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XVSHNPostgreSQLSpec   `json:"spec"`
	Status XVSHNPostgreSQLStatus `json:"status,omitempty"`
}

type XVSHNPostgreSQLStatus struct {
	VSHNPostgreSQLStatus `json:",inline"`
	xpv1.ResourceStatus  `json:",inline"`
}

type XVSHNPostgreSQLSpec struct {
	// Parameters are the configurable fields of a VSHNPostgreSQL.
	Parameters        VSHNPostgreSQLParameters `json:"parameters,omitempty"`
	xpv1.ResourceSpec `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNPostgreSQLList represents a list of composites
type XVSHNPostgreSQLList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XVSHNPostgreSQL `json:"items"`
}

func (v *VSHNPostgreSQL) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-postgresql-%s", v.GetName())
}

func (v *VSHNPostgreSQL) SetInstanceNamespaceStatus() {
	v.Status.InstanceNamespace = v.GetInstanceNamespace()
}

func (v *XVSHNPostgreSQL) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-postgresql-%s", v.GetName())
}

// GetBackupRetention returns the retention definition for this backup.
// !!! This is just a placeholder to satisfy InfoGetter interface
func (v *VSHNPostgreSQL) GetBackupRetention() K8upRetentionPolicy {
	return K8upRetentionPolicy{}
}

// GetServiceName returns the name of this service
func (v *VSHNPostgreSQL) GetServiceName() string {
	return "postgresql"
}

// GetAllowAllNamespaces returns the AllowAllNamespaces field of this service
func (v *VSHNPostgreSQL) GetAllowAllNamespaces() bool {
	return v.Spec.Parameters.Security.AllowAllNamespaces
}

// GetAllowedNamespaces returns the AllowedNamespaces array of this service
func (v *VSHNPostgreSQL) GetAllowedNamespaces() []string {
	if v.Spec.Parameters.Security.AllowedNamespaces == nil {
		v.Spec.Parameters.Security.AllowedNamespaces = []string{}
	}
	return append(v.Spec.Parameters.Security.AllowedNamespaces, v.GetClaimNamespace())
}

func (v *VSHNPostgreSQL) GetSize() VSHNSizeSpec {
	return v.Spec.Parameters.Size
}

func (v *VSHNPostgreSQL) GetMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

func (v *VSHNPostgreSQL) GetInstances() int {
	return v.Spec.Parameters.Instances
}

func (v *VSHNPostgreSQL) GetPDBLabels() map[string]string {
	return map[string]string{
		"app": "StackGresCluster",
	}
}
