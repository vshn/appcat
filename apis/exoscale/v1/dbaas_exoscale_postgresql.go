package v1

import (
	v1 "github.com/vshn/appcat/v4/apis/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/exoscale.appcat.vshn.io_exoscalepostgresqls.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/exoscale.appcat.vshn.io_exoscalepostgresqls.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.maintenance.default={})"
//go:generate yq -i e ../../generated/exoscale.appcat.vshn.io_exoscalepostgresqls.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.default={})"
//go:generate yq -i e ../../generated/exoscale.appcat.vshn.io_exoscalepostgresqls.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"
//go:generate yq -i e ../../generated/exoscale.appcat.vshn.io_exoscalepostgresqls.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/exoscale.appcat.vshn.io_exoscalepostgresqls.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.network.default={})"

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Plan",type="string",JSONPath=".spec.parameters.size.plan"
// +kubebuilder:printcolumn:name="Zone",type="string",JSONPath=".spec.parameters.service.zone"

// ExoscalePostgreSQL is the API for creating PostgreSQL on Exoscale.
type ExoscalePostgreSQL struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a ExoscalePostgreSQL.
	Spec ExoscalePostgreSQLSpec `json:"spec,omitempty"`
	// Status reflects the observed state of a ExoscalePostgreSQL.
	Status ExoscalePostgreSQLStatus `json:"status,omitempty"`
}

type ExoscalePostgreSQLSpec struct {
	// Parameters are the configurable fields of a ExoscalePostgreSQL.
	Parameters ExoscalePostgreSQLParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef v1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

type ExoscalePostgreSQLParameters struct {
	// Service contains Exoscale PostgreSQL DBaaS specific properties
	Service ExoscalePostgreSQLServiceSpec `json:"service,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance ExoscaleDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size ExoscaleDBaaSSizeSpec `json:"size,omitempty"`

	// Network contains any network related settings.
	Network ExoscaleDBaaSNetworkSpec `json:"network,omitempty"`

	// Backup contains settings to control the backups of an instance.
	Backup ExoscaleDBaaSBackupSpec `json:"backup,omitempty"`
}

type ExoscalePostgreSQLServiceSpec struct {
	ExoscaleDBaaSServiceSpec `json:",inline"`

	// +kubebuilder:validation:Enum="14"
	// +kubebuilder:default="14"

	// MajorVersion contains the major version for PostgreSQL.
	// Currently only "14" is supported. Leave it empty to always get the latest supported version.
	MajorVersion string `json:"majorVersion,omitempty"`

	// PGSettings contains additional PostgreSQL settings.
	PostgreSQLSettings runtime.RawExtension `json:"pgSettings,omitempty"`
}

type ExoscalePostgreSQLStatus struct {
	// PostgreSQLConditions contains the status conditions of the backing object.
	PostgreSQLConditions []v1.Condition `json:"postgresqlConditions,omitempty"`
}
