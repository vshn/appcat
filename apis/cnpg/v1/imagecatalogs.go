package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

// ImageCatalog is the API for creating CNPG image catalogs.
type ImageCatalog struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a ImageCatalog.
	Spec ImageCatalogSpec `json:"spec"`
}

// +kubebuilder:object:root=true

type ImageCatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ImageCatalog `json:"items"`
}
