package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

type CompositeMariaDBInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CompositeMariaDBInstanceSpec   `json:"spec"`
	Status CompositeMariaDBInstanceStatus `json:"status,omitempty"`
}

type CompositeMariaDBInstanceStatus struct {
	xpv1.ResourceStatus `json:",inline"`
}

type CompositeMariaDBInstanceSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	Parameters        CompositeMariadbInstanceParameters `json:"parameters,omitempty"`
}

type CompositeMariadbInstanceParameters struct {
	// Enable or disable TLS for this instance.
	TLS bool `json:"tls,omitempty"`
	// Enforce TLS on this instance. Needs tls to be true.
	RequireTLS bool `json:"requireTLS,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// CompositeMariaDBInstanceList represents a list of composites
type CompositeMariaDBInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CompositeMariaDBInstance `json:"items"`
}
