package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

// SGObjectStorage is the API for creating SgObjectStorage objects.
type SGObjectStorage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a SGObjectStorage.
	Spec SGObjectStorageSpec `json:"spec"`
}

// +kubebuilder:object:root=true

type SGObjectStorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SGObjectStorage `json:"items"`
}
