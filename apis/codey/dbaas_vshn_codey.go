package codey

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate yq -i e ../generated/codey.io_instances.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"

// +groupName=codey.io
// +versionName=v1
// +kubebuilder:object:root=true

// Instance is the API for creating Instance instances.
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a Instance.
	Spec InstanceSpec `json:"spec"`

	// Status reflects the observed state of a Instance.
	Status InstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type InstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Instance `json:"items,omitempty"`
}

// InstanceSpec defines the desired state of a Instance.
type InstanceSpec struct {
	// Parameters are the configurable fields of a Instance.
	// +kubebuilder:default={}
	Parameters InstanceParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef xpv1.LocalSecretReference `json:"writeConnectionSecretToRef,omitempty"`
}

// InstanceParameters are the configurable fields of a Instance.
type InstanceParameters struct {
	// Service contains Instance DBaaS specific properties
	Service InstanceServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNInstanceSizeSpec `json:"size,omitempty"`
}

// InstanceServiceSpec contains Instance DBaaS specific properties
type InstanceServiceSpec struct {

	// Version contains supported version of Instance.
	// Multiple versions are supported.
	MajorVersion string `json:"majorVersion,omitempty"`

	// AdminEmail for email notifications.
	// +kubebuilder:validation:Required
	AdminEmail string `json:"adminEmail"`

	// FQDN contains the FQDNs array, which will be used for the ingress.
	// If it's not set, no ingress will be deployed.
	// This also enables strict hostname checking for this FQDN.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	FQDN []string `json:"fqdn"`
}

// InstanceStatus reflects the observed state of a Instance.
type InstanceStatus struct {
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XInstance represents the internal composite of this claim
type XInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XInstanceSpec   `json:"spec"`
	Status XInstanceStatus `json:"status,omitempty"`
}

// XInstanceSpec defines the desired state of a Instance.
type XInstanceSpec struct {
	// Parameters are the configurable fields of a Instance.
	Parameters InstanceParameters `json:"parameters,omitempty"`

	xpv1.ResourceSpec `json:",inline"`
}

type XInstanceStatus struct {
	InstanceStatus      `json:",inline"`
	xpv1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XInstanceList represents a list of composites
type XInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XInstance `json:"items"`
}

// VSHNInstanceSizeSpec contains settings to control the sizing of a service.
type VSHNInstanceSizeSpec struct {
	// Size contains settings to control the sizing of a service.
	// +kubebuilder:validation:Enum=mini;small
	// +kubebuilder:default=mini
	// Plan is the name of the resource plan that defines the compute resources.
	Plan string `json:"plan,omitempty"`
}
