package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// The reason we need a custom condition type is that metav1.Condition declares all fields as required.
// However, we have seen issues where Crossplane's properties in Conditions aren't all required and lead to Crossplane not being able to copy conditions.

type Condition struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`
	// +kubebuilder:validation:MaxLength=316

	// Type of condition.
	Type string `json:"type,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=True;False;Unknown

	// Status of the condition, one of True, False, Unknown.
	Status metav1.ConditionStatus `json:"status,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0

	// ObservedGeneration represents the .metadata.generation that the condition was set based upon.
	// For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date with respect to the current state of the instance.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=date-time

	// LastTransitionTime is the last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:Pattern=`^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`

	// Reason contains a programmatic identifier indicating the reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=32768

	// Message is a human-readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
}
