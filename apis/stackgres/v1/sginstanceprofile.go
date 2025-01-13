package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

// SGInstanceProfile is the API for creating instance profiles for SgClusters.
type SGInstanceProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec contains the custom configurations for the SgInstanceProfile.
	Spec SGInstanceProfileSpec `json:"spec"`
}

// +kubebuilder:object:root=true

type SGInstanceProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SGInstanceProfile `json:"items"`
}
