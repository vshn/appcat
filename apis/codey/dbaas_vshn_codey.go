package codey

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate yq -i e ../generated/codey.io_codeyinstances.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"

// +groupName=codey.io
// +versionName=v1
// +kubebuilder:object:root=true

// CodeyInstance is the API for creating CodeyInstance instances.
type CodeyInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a CodeyCodeyInstance.
	Spec CodeyInstanceSpec `json:"spec"`

	// Status reflects the observed state of a CodeyInstance.
	Status CodeyInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type CodeyInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CodeyInstance `json:"items,omitempty"`
}

// CodeyInstanceSpec defines the desired state of a CodeyInstance.
type CodeyInstanceSpec struct {
	// Parameters are the configurable fields of a CodeyInstance.
	// +kubebuilder:default={}
	Parameters CodeyInstanceParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef xpv1.LocalSecretReference `json:"writeConnectionSecretToRef,omitempty"`
}

// CodeyInstanceParameters are the configurable fields of a CodeyInstance.
type CodeyInstanceParameters struct {
	// Service contains CodeyInstance DBaaS specific properties
	Service CodeyInstanceServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNCodeyInstanceSizeSpec `json:"size,omitempty"`
}

// CodeyInstanceServiceSpec contains CodeyInstance DBaaS specific properties
type CodeyInstanceServiceSpec struct {

	// Version contains supported version of CodeyInstance.
	// Multiple versions are supported.
	MajorVersion string `json:"majorVersion,omitempty"`

	// AdminEmail for email notifications.
	// +kubebuilder:validation:Required
	AdminEmail string `json:"adminEmail"`
}

// CodeyInstanceStatus reflects the observed state of a CodeyInstance.
type CodeyInstanceStatus struct {
	// CodeyInstanceNamespace contains the name of the namespace where the instance resides
	CodeyInstanceNamespace string `json:"instanceNamespace,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XCodeyInstance represents the internal composite of this claim
type XCodeyInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XCodeyInstanceSpec   `json:"spec"`
	Status XCodeyInstanceStatus `json:"status,omitempty"`
}

// XCodeyInstanceSpec defines the desired state of a CodeyInstance.
type XCodeyInstanceSpec struct {
	// Parameters are the configurable fields of a CodeyInstance.
	Parameters CodeyInstanceParameters `json:"parameters,omitempty"`

	xpv1.ResourceSpec `json:",inline"`
}

type XCodeyInstanceStatus struct {
	CodeyInstanceStatus `json:",inline"`
	xpv1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XCodeyInstanceList represents a list of composites
type XCodeyInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XCodeyInstance `json:"items"`
}

// VSHNCodeyInstanceSizeSpec contains settings to control the sizing of a service.
type VSHNCodeyInstanceSizeSpec struct {
	// Size contains settings to control the sizing of a service.
	// +kubebuilder:validation:Enum=mini;small
	// +kubebuilder:default=mini
	// Plan is the name of the resource plan that defines the compute resources.
	Plan string `json:"plan,omitempty"`
}
