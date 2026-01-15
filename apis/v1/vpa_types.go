package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.spec.selector
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.replicas"

// VPA is a simple Custom Resource Definition with scale subresource support
type VPA struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VPASpec   `json:"spec"`
	Status VPAStatus `json:"status,omitempty"`
}

// VPASpec defines the desired state of VPA
type VPASpec struct {
	// Replicas is the desired number of replicas
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`
	// Selector is the label selector for pods (used by scale subresource)
	// +optional
	Selector string `json:"selector,omitempty"`
}

// VPAStatus reflects the observed state of VPA
type VPAStatus struct {
	// Replicas is the actual number of replicas
	Replicas int32 `json:"replicas,omitempty"`
}

// +kubebuilder:object:root=true

// VPAList contains a list of VPA
type VPAList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPA `json:"items"`
}
