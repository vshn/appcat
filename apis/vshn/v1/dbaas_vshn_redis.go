package v1

import (
	v1 "github.com/vshn/appcat/v4/apis/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnredis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnredis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnredis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnredis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.tls.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnredis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.default={})"

// +kubebuilder:object:root=true

// VSHNRedis is the API for creating Redis clusters.
type VSHNRedis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNRedis.
	Spec VSHNRedisSpec `json:"spec"`

	// Status reflects the observed state of a VSHNRedis.
	Status VSHNRedisStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type VSHNRedisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNRedis `json:"items,omitempty"`
}

// VSHNRedisSpec defines the desired state of a VSHNRedis.
type VSHNRedisSpec struct {
	// Parameters are the configurable fields of a VSHNRedis.
	Parameters VSHNRedisParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef v1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

// VSHNRedisParameters are the configurable fields of a VSHNRedis.
type VSHNRedisParameters struct {
	// Service contains Redis DBaaS specific properties
	Service VSHNRedisServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNRedisSizeSpec `json:"size,omitempty"`

	// Scheduling contains settings to control the scheduling of an instance.
	Scheduling VSHNDBaaSSchedulingSpec `json:"scheduling,omitempty"`

	// TLS contains settings to control tls traffic of a service.
	TLS VSHNRedisTLSSpec `json:"tls,omitempty"`

	// Backup contains settings to control how the instance should get backed up.
	Backup K8upBackupSpec `json:"backup,omitempty"`

	// Restore contains settings to control the restore of an instance.
	Restore K8upRestoreSpec `json:"restore,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance VSHNDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`
}

// VSHNRedisServiceSpec contains Redis DBaaS specific properties
type VSHNRedisServiceSpec struct {
	// +kubebuilder:validation:Enum="6.2";"7.0"
	// +kubebuilder:default="7.0"

	// Version contains supported version of Redis.
	// Multiple versions are supported. The latest version "7.0" is the default version.
	Version string `json:"version,omitempty"`

	// RedisSettings contains additional Redis settings.
	RedisSettings string `json:"redisSettings,omitempty"`
}

// VSHNRedisSizeSpec contains settings to control the sizing of a service.
type VSHNRedisSizeSpec struct {

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

// VSHNRedisTLSSpec contains settings to control tls traffic of a service.
type VSHNRedisTLSSpec struct {
	// +kubebuilder:default=true

	// TLSEnabled enables TLS traffic for the service
	TLSEnabled bool `json:"enabled,omitempty"`

	// +kubebuilder:default=true
	// TLSAuthClients enables client authentication requirement
	TLSAuthClients bool `json:"authClients,omitempty"`
}

// VSHNRedisStatus reflects the observed state of a VSHNRedis.
type VSHNRedisStatus struct {
	// RedisConditions contains the status conditions of the backing object.
	NamespaceConditions         []v1.Condition `json:"namespaceConditions,omitempty"`
	SelfSignedIssuerConditions  []v1.Condition `json:"selfSignedIssuerConditions,omitempty"`
	LocalCAConditions           []v1.Condition `json:"localCAConditions,omitempty"`
	CaCertificateConditions     []v1.Condition `json:"caCertificateConditions,omitempty"`
	ServerCertificateConditions []v1.Condition `json:"serverCertificateConditions,omitempty"`
	ClientCertificateConditions []v1.Condition `json:"clientCertificateConditions,omitempty"`
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNRedis represents the internal composite of this claim
type XVSHNRedis VSHNRedis

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNRedisList represents a list of composites
type XVSHNRedisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XVSHNRedis `json:"items"`
}
