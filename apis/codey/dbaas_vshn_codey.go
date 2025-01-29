package codey

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +groupName=codey.io
// +versionName=v1
// +kubebuilder:object:root=true

// Codey is the API for creating Codey instances.
type Codey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a Codey.
	Spec CodeySpec `json:"spec"`

	// Status reflects the observed state of a Codey.
	Status CodeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type CodeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Codey `json:"items,omitempty"`
}

// CodeySpec defines the desired state of a Codey.
type CodeySpec struct {
	// Parameters are the configurable fields of a Codey.
	// +kubebuilder:default={}
	Parameters CodeyParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef xpv1.LocalSecretReference `json:"writeConnectionSecretToRef,omitempty"`
}

// CodeyParameters are the configurable fields of a Codey.
type CodeyParameters struct {
	// Service contains Codey DBaaS specific properties
	Service CodeyServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	// +kubebuilder:validation:Enum=mini;small
	// +kubebuilder:default=mini
	Size string `json:"size,omitempty"`
}

// CodeyServiceSpec contains Codey DBaaS specific properties
type CodeyServiceSpec struct {

	// Version contains supported version of Codey.
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

// CodeyStatus reflects the observed state of a Codey.
type CodeyStatus struct {
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XCodey represents the internal composite of this claim
type XCodey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XCodeySpec   `json:"spec"`
	Status XCodeyStatus `json:"status,omitempty"`
}

// XCodeySpec defines the desired state of a Codey.
type XCodeySpec struct {
	// Parameters are the configurable fields of a Codey.
	Parameters CodeyParameters `json:"parameters,omitempty"`

	xpv1.ResourceSpec `json:",inline"`
}

type XCodeyStatus struct {
	CodeyStatus         `json:",inline"`
	xpv1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XCodeyList represents a list of composites
type XCodeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XCodey `json:"items"`
}
