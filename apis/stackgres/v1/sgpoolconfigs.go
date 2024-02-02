package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

// SGPoolingConfig is the API for creating pgbouncer configs clusters.
type SGPoolingConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec contains the custom configurations for the pgbouncer.
	Spec SGPoolingConfigSpec `json:"spec"`

	// Status contains the default settings for the pgbouncer.
	Status SGPoolingConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type SGPoolingConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SGPoolingConfig `json:"items"`
}
