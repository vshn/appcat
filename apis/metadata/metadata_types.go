package metadata

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:skip

// MetadataOnlyObject allows decoding only the apiVersion, kind, and metadata fields of
// JSON data.
//
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type MetadataOnlyObject struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// MetadataOnlyObjectList allows decoding from JSON data only the typemeta and metadata of
// a list, and those of the enclosing objects.
//
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type MetadataOnlyObjectList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []MetadataOnlyObject `json:"items"`
}
