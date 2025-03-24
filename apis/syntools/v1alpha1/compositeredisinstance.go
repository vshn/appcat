package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

type CompositeRedisInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CompositeRedisInstanceSpec   `json:"spec"`
	Status CompositeRedisInstanceStatus `json:"status,omitempty"`
}

type CompositeRedisInstanceStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	ReconcileTimeStamp  string `json:"reconcileTimeStamp,omitempty"`
}

type CompositeRedisInstanceSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	Parameters        CompositeRedisInstanceParameters `json:"parameters,omitempty"`
}

type CompositeRedisInstanceParameters struct {
	// Enable or disable TLS for this instance.
	TLS bool `json:"tls,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// CompositeRedisInstanceList represents a list of composites
type CompositeRedisInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CompositeRedisInstance `json:"items"`
}
