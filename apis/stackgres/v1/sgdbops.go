package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const SGDbOpsOpRestart = "restart"

const SGDbOpsRestartMethodInPlace = "InPlace"

// +kubebuilder:object:root=true

// VSHNPostgreSQL is the API for creating Postgresql clusters.
type SGDbOps struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNPostgreSQL.
	Spec SGDbOpsSpec `json:"spec"`

	// Status reflects the observed state of a VSHNPostgreSQL.
	Status SGDbOpsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type SGDbOpsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SGDbOps `json:"items"`
}
