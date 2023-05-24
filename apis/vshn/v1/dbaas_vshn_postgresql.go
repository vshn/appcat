package v1

import (
	alertmanagerv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	v1 "github.com/vshn/appcat/apis/v1"
	corev1 "k8s.io/api/core/v1"
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
	Parameters VSHNPostgreSQLParameters `json:"parameters,omitempty"`
	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef v1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`

	// ResourceRef contains a reference to the composite.
	ResourceRef corev1.ObjectReference `json:"resourceRef,omitempty"`

	// CompositeDeletePolicy defines how the claim should behave if it's deleted.
	// This field definition will be overwritten by crossplane again, once the XRD is applied to a cluster.
	// It's added here so it can be marshalled correctly in third party operators or composition functions.
	CompositeDeletePolicy string `json:"compositeDeletePolicy,omitempty"`
}

// VSHNPostgreSQLParameters are the configurable fields of a VSHNPostgreSQL.
type VSHNPostgreSQLParameters struct {
	// Service contains PostgreSQL DBaaS specific properties
	Service VSHNPostgreSQLServiceSpec `json:"service,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance VSHNDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNDBaaSSizeSpec `json:"size,omitempty"`

	// Scheduling contains settings to control the scheduling of an instance.
	Scheduling VSHNDBaaSSchedulingSpec `json:"scheduling,omitempty"`

	// Network contains any network related settings.
	Network VSHNDBaaSNetworkSpec `json:"network,omitempty"`

	// Backup contains settings to control the backups of an instance.
	Backup VSHNPostgreSQLBackup `json:"backup,omitempty"`

	// Restore contains settings to control the restore of an instance.
	Restore VSHNPostgreSQLRestore `json:"restore,omitempty"`

	// Monitoring contains settings to control monitoring.
	Monitoring VSHNPostgreSQLMonitoring `json:"monitoring,omitempty"`

	// Encryption contains settings to control the storage encryption of an instance.
	Encryption VSHNPostgreSQLEncryption `json:"encryption,omitempty"`

	// UpdateStrategy indicates when updates to the instance spec will be applied.
	UpdateStrategy VSHNPostgreSQLUpdateStrategy `json:"updateStrategy,omitempty"`
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
}

// VSHNDBaaSSchedulingSpec contains settings to control the scheduling of an instance.
type VSHNDBaaSSchedulingSpec struct {
	// NodeSelector is a selector which must match a node’s labels for the pod to be scheduled on that node
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

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

// VSHNDBaaSSizeSpec contains settings to control the sizing of a service.
type VSHNDBaaSSizeSpec struct {
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
	DeletionProtection bool `json:"deletionProtection,omitempty"`

	// DeletionRetention specifies in days how long the instance should be kept after deletion.
	// The default is keeping it one week.
	// +kubebuilder:default=7
	DeletionRetention int `json:"deletionRetention,omitempty"`
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

// VSHNPostgreSQLMonitoring contains settings to configure monitoring aspects of PostgreSQL
type VSHNPostgreSQLMonitoring struct {
	// AlertmanagerConfigRef contains the name of the AlertmanagerConfig that should be copied over to the
	// namespace of the PostgreSQL instance.
	AlertmanagerConfigRef string `json:"alertmanagerConfigRef,omitempty"`

	// AlertmanagerConfigSecretRef contains the name of the secret that is used
	// in the referenced AlertmanagerConfig
	AlertmanagerConfigSecretRef string `json:"alertmanagerConfigSecretRef,omitempty"`

	// AlertmanagerConfigSpecTemplate takes an AlertmanagerConfigSpec object.
	// This takes precedence over the AlertmanagerConfigRef.
	AlertmanagerConfigSpecTemplate *alertmanagerv1alpha1.AlertmanagerConfigSpec `json:"alertmanagerConfigTemplate,omitempty"`
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
	ObjectBucketConditions       []v1.Condition `json:"ObjectBucketConditions,omitempty"`
	ObjectBackupConfigConditions []v1.Condition `json:"ObjectBackupConfigConditions,omitempty"`
	NetworkPolicyConditions      []v1.Condition `json:"networkPolicyConditions,omitempty"`
	LocalCAConditions            []v1.Condition `json:"localCAConditions,omitempty"`
	CertificateConditions        []v1.Condition `json:"certificateConditions,omitempty"`
	// IsEOL indicates if this instance is using an EOL version of PostgreSQL.
	IsEOL bool `json:"isEOL,omitempty"`
}

// +kubebuilder:object:root=true

// VSHNPostgreSQLList defines a list of VSHNPostgreSQL
type VSHNPostgreSQLList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNPostgreSQL `json:"items"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNPostgreSQL represents the internal composite of this claim
type XVSHNPostgreSQL VSHNPostgreSQL

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNPostgreSQLList represents a list of composites
type XVSHNPostgreSQLList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XVSHNPostgreSQL `json:"items"`
}
