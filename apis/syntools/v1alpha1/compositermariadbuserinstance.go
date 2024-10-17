package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

type CompositeMariaDBUserInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CompositeMariaDBUserInstanceSpec `json:"spec"`
	Status CompositeMariaDBInstanceStatus   `json:"status,omitempty"`
}

type CompositeMariaDBUserInstanceStatus struct {
	xpv1.ResourceStatus `json:",inline"`
}

type CompositeMariaDBUserInstanceSpec struct {
	xpv1.ResourceSpec `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// CompositeMariaDBUserInstanceList represents a list of composites
type CompositeMariaDBUserInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CompositeMariaDBUserInstance `json:"items"`
}
