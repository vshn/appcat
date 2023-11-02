package v1

import (
	v1 "github.com/vshn/appcat/v4/apis/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/aws.appcat.vshn.io_awsrds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/aws.appcat.vshn.io_awsrds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.maintenance.default={})"
//go:generate yq -i e ../../generated/aws.appcat.vshn.io_awsrds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.default={})"
//go:generate yq -i e ../../generated/aws.appcat.vshn.io_awsrds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"
//go:generate yq -i e ../../generated/aws.appcat.vshn.io_awsrds.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Plan",type="string",JSONPath=".spec.parameters.size.plan"
// +kubebuilder:printcolumn:name="Region",type="string",JSONPath=".spec.parameters.service.region"

// AwsRds is the API for creating RDS instances on AWS.
type AwsRds struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a AwsRds.
	Spec AwsRdsSpec `json:"spec,omitempty"`
	// Status reflects the observed state of a AwsRds.
	Status AwsRdsStatus `json:"status,omitempty"`
}

type AwsRdsSpec struct {
	// Parameters are the configurable fields of a AwsRds.
	Parameters AwsRdsParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef v1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

type AwsRdsParameters struct {
	// Service contains AWS RDS DBaaS specific properties
	Service AwsRdsServiceSpec `json:"service,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance AwsDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size AwsDBaaSSizeSpec `json:"size,omitempty"`

	// Backup contains settings to control the backups of an instance.
	Backup AwsDBaaSBackupSpec `json:"backup,omitempty"`
}

type AwsRdsServiceSpec struct {
	AwsDBaaSServiceSpec `json:",inline"`

	// +kubebuilder:validation:Enum=aurora-mysql;aurora-postgresql;custom-oracle-ee;custom-oracle-ee-cdb;custom-sqlserver-ee;custom-sqlserver-se;custom-sqlserver-web;mariadb;mysql;oracle-ee;oracle-ee-cdb;oracle-se2;oracle-se2-cdb;postgres;sqlserver-ee;sqlserver-se;sqlserver-ex;sqlserver-web
	// +kubebuilder:default="mysql"

	// Engine contains the type of the DB instance class.
	Engine string `json:"engine,omitempty"`

	// +kubebuilder:default="8.0"

	// MajorVersion contains the major version for the instance.
	// Depends on the chosen engine.
	MajorVersion string `json:"majorVersion,omitempty"`

	// RdsSettings contains additional settings for the RDS instance.
	RdsSettings []ParameterParameters `json:"rdsSettings,omitempty"`

	// RdsOptions contains additional options for the RDS instance.
	RdsOptions []OptionObservation `json:"rdsOptions,omitempty"`

	// +kubebuilder:default="adminUser"

	// AdminUser contains the username for the admin user.
	AdminUser string `json:"adminUser,omitempty"`

	// DBName contains the name of the database to create.
	DBName string `json:"dbName,omitempty"`
}

type AwsRdsStatus struct {
	// RdsConditions contains the status conditions of the backing object.
	RdsConditions []v1.Condition `json:"rdsConditions,omitempty"`
}

type OptionObservation struct {

	// The Name of the Option (e.g., MEMCACHED).
	OptionName *string `json:"optionName,omitempty"`

	// A list of option settings to apply.
	OptionSettings []OptionSettingsObservation `json:"optionSettings,omitempty"`

	// The Port number when connecting to the Option (e.g., 11211).
	Port int `json:"port,omitempty"`

	// The version of the option (e.g., 13.1.0.0).
	Version string `json:"version,omitempty"`
}

type OptionSettingsObservation struct {

	// The name of the option group. Must be lowercase, to match as it is stored in AWS.
	Name string `json:"name,omitempty"`

	// The Value of the setting.
	Value string `json:"value,omitempty"`
}

type ParameterParameters struct {

	// "immediate" (default), or "pending-reboot". Some
	// engines can't apply some parameters without a reboot, and you will need to
	// specify "pending-reboot" here.
	// +kubebuilder:validation:Optional
	ApplyMethod string `json:"applyMethod,omitempty"`

	// The name of the DB parameter group.
	// +kubebuilder:validation:Optional
	Name string `json:"name"`

	// The value of the DB parameter.
	// +kubebuilder:validation:Optional
	Value string `json:"value" tf:"value,omitempty"`
}
