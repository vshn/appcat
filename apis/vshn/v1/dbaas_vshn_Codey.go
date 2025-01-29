package v1

import (
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshncodeys.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"

// +kubebuilder:object:root=true

// VSHNCodey is the API for creating VSHNCodey instances.
type VSHNCodey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNCodey.
	Spec VSHNCodeySpec `json:"spec"`

	// Status reflects the observed state of a VSHNCodey.
	Status VSHNCodeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type VSHNCodeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNCodey `json:"items,omitempty"`
}

// VSHNCodeySpec defines the desired state of a VSHNCodey.
type VSHNCodeySpec struct {
	// Parameters are the configurable fields of a VSHNCodey.
	Parameters VSHNCodeyParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

// VSHNCodeyParameters are the configurable fields of a VSHNCodey.
type VSHNCodeyParameters struct {
	// Service contains VSHNCodey DBaaS specific properties
	Service VSHNCodeyServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	// +kubebuilder:validation:Enum=mini;small
	// +kubebuilder:default=mini
	Size string `json:"size,omitempty"`
}

// VSHNCodeyServiceSpec contains VSHNCodey DBaaS specific properties
type VSHNCodeyServiceSpec struct {

	// Version contains supported version of VSHNCodey.
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

// VSHNCodeyStatus reflects the observed state of a VSHNCodey.
type VSHNCodeyStatus struct {
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
	// Schedules keeps track of random generated schedules, is overwriten by
	// schedules set in the service's spec.
	Schedules VSHNScheduleStatus `json:"schedules,omitempty"`
}

func (v *VSHNCodey) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

func (v *VSHNCodey) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-codey-%s", v.GetName())
}

func (v *VSHNCodey) SetInstanceNamespaceStatus() {
	v.Status.InstanceNamespace = v.GetInstanceNamespace()
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

// XCodeySpec defines the desired state of a VSHNCodey.
type XCodeySpec struct {
	// Parameters are the configurable fields of a VSHNCodey.
	Parameters VSHNCodeyParameters `json:"parameters,omitempty"`

	xpv1.ResourceSpec `json:",inline"`
}

type XCodeyStatus struct {
	VSHNCodeyStatus     `json:",inline"`
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
