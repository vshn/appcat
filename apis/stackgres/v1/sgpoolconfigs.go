package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

// SGPoolingConfig is the API for creating pgbouncer configs clusters.
type SGPoolingConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNPostgreSQL.
	Spec SGPoolingConfigSpec `json:"spec"`

	// Status reflects the observed state of a VSHNPostgreSQL.
	Status SGPoolingConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type SGPoolingConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SGPoolingConfig `json:"items"`
}
