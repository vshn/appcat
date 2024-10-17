package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

type CompositeMariaDBDatabaseInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CompositeMariaDBUserInstanceSpec `json:"spec"`
	Status CompositeMariaDBInstanceStatus   `json:"status,omitempty"`
}

type CompositeMariaDBDatabaseInstanceStatus struct {
	xpv1.ResourceStatus `json:",inline"`
}

type CompositeMariaDBDatabaseInstanceSpec struct {
	xpv1.ResourceSpec `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// CompositeMariaDBUserInstanceList represents a list of composites
type CompositeMariaDBDatabaseInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CompositeMariaDBDatabaseInstance `json:"items"`
}
