package v1

// controller-gen cannot handle the CNPG cluster CRD, it has too many nested anonymous structs.
// As such, the cluster object is partially reconstructed, only containing fields that we are interested in.

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

// Cluster is the API for creating CNPG clusters.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a Cluster.
	Spec ClusterSpec `json:"spec"`

	// Status defines the status of a Cluster.
	Status ClusterStatus `json:"status"`
}

// +kubebuilder:object:root=true

type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Cluster `json:"items"`
}

type ClusterSpec struct {
	// Defines the major PostgreSQL version we want to use within an ImageCatalog
	ImageCatalogRef *ClusterSpecImageCatalogRef `json:"imageCatalogRef,omitempty"`

	// Name of the container image, supporting both tags (`<image>:<tag>`)
	// and digests for deterministic and repeatable deployments
	// (`<image>:<tag>@sha256:<digestValue>`)
	ImageName *string `json:"imageName,omitempty"`

	// Number of instances required in the cluster
	Instances int `json:"instances"`
}

// ClusterSpecImageCatalogRef defines model for ClusterSpecImageCatalogRef.
type ClusterSpecImageCatalogRef struct {
	// APIGroup is the group for the resource being referenced.
	// If APIGroup is not specified, the specified Kind must be in the core API group.
	// For any other third-party types, APIGroup is required.
	ApiGroup *string `json:"apiGroup,omitempty"`

	// Kind is the type of resource being referenced
	Kind string `json:"kind"`

	// The major version of PostgreSQL we want to use from the ImageCatalog
	Major int `json:"major"`

	// Name is the name of resource being referenced
	Name string `json:"name"`
}

type ClusterStatus struct {
	Instances      int    `json:"instances"`
	ReadyInstances int    `json:"readyInstances"`
	Image          string `json:"image"`
}
