package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const SGClusterConditionTypePendingRestart = "PendingRestart"
const SGClusterConditionTypePendingUpgrade = "PendingUpgrade"

// +kubebuilder:object:root=true

// SGCluster is the API for creating Postgresql clusters.
type SGCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNPostgreSQL.
	Spec SGClusterSpec `json:"spec"`

	// Status reflects the observed state of a VSHNPostgreSQL.
	Status SGClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type SGClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SGCluster `json:"items"`
}
