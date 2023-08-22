package v1

import (
	v1 "github.com/vshn/appcat/v4/apis/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/exoscale.appcat.vshn.io_exoscaleredis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/exoscale.appcat.vshn.io_exoscaleredis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.maintenance.default={})"
//go:generate yq -i e ../../generated/exoscale.appcat.vshn.io_exoscaleredis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"
//go:generate yq -i e ../../generated/exoscale.appcat.vshn.io_exoscaleredis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/exoscale.appcat.vshn.io_exoscaleredis.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.network.default={})"

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Plan",type="string",JSONPath=".spec.parameters.size.plan"
// +kubebuilder:printcolumn:name="Zone",type="string",JSONPath=".spec.parameters.service.zone"

// ExoscaleRedis is the API for creating Redis instances on Exoscale.
type ExoscaleRedis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a ExoscaleRedis.
	Spec ExoscaleRedisSpec `json:"spec,omitempty"`
	// Status reflects the observed state of a ExoscaleRedis.
	Status ExoscaleRedisStatus `json:"status,omitempty"`
}

type ExoscaleRedisSpec struct {
	// Parameters are the configurable fields of a ExoscaleRedis.
	Parameters ExoscaleRedisParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef v1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

type ExoscaleRedisParameters struct {
	// Service contains Exoscale Redis DBaaS specific properties
	Service ExoscaleRedisServiceSpec `json:"service,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance ExoscaleDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size ExoscaleDBaaSSizeSpec `json:"size,omitempty"`

	// Network contains any network related settings.
	Network ExoscaleDBaaSNetworkSpec `json:"network,omitempty"`
}

type ExoscaleRedisServiceSpec struct {
	ExoscaleDBaaSServiceSpec `json:",inline"`
	// RedisSettings contains additional Redis settings.
	RedisSettings runtime.RawExtension `json:"redisSettings,omitempty"`
}

type ExoscaleRedisStatus struct {
	// RedisConditions contains the status conditions of the backing object.
	RedisConditions []v1.Condition `json:"redisConditions,omitempty"`
}
