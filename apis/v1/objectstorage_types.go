package v1

import (
	crossplane "github.com/crossplane/crossplane/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DeleteIfEmpty only deletes the bucket if the bucket is empty.
	DeleteIfEmpty BucketDeletionPolicy = "DeleteIfEmpty"
	// DeleteAll recursively deletes all objects in the bucket and then removes it.
	DeleteAll BucketDeletionPolicy = "DeleteAll"
)

// BucketDeletionPolicy determines how buckets should be deleted when a Bucket is deleted.
type BucketDeletionPolicy string

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Bucket Name",type="string",JSONPath=".spec.parameters.bucketName"
// +kubebuilder:printcolumn:name="Region",type="string",JSONPath=".spec.parameters.region"

// ObjectBucket is the API for creating S3 buckets.
type ObjectBucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectBucketSpec   `json:"spec"`
	Status ObjectBucketStatus `json:"status,omitempty"`
}

// ObjectBucketSpec defines the desired state of a ObjectBucket.
type ObjectBucketSpec struct {
	Parameters ObjectBucketParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef LocalObjectReference            `json:"writeConnectionSecretToRef,omitempty"`
	CompositionReference       crossplane.CompositionReference `json:"compositionRef,omitempty"`
}

// ObjectBucketParameters are the configurable fields of a ObjectBucket.
type ObjectBucketParameters struct {
	// +kubebuilder:validation:Required

	// BucketName is the name of the bucket to create.
	// Cannot be changed after bucket is created.
	// Name must be acceptable by the S3 protocol, which follows RFC 1123.
	// Be aware that S3 providers may require a unique name across the platform or region.
	BucketName string `json:"bucketName"`

	// +kubebuilder:validation:Required

	// Region is the name of the region where the bucket shall be created.
	// The region must be available in the S3 endpoint.
	Region string `json:"region"`

	// +kubebuilder:default=DeleteAll

	// BucketDeletionPolicy determines how buckets should be deleted when Bucket is deleted.
	//  `DeleteIfEmpty` only deletes the bucket if the bucket is empty.
	//  `DeleteAll` recursively deletes all objects in the bucket and then removes it.
	BucketDeletionPolicy BucketDeletionPolicy `json:"bucketDeletionPolicy,omitempty"`
}

// ObjectBucketStatus reflects the observed state of a ObjectBucket.
type ObjectBucketStatus struct {
	// AccessUserConditions contains a copy of the claim's underlying user account conditions.
	AccessUserConditions []Condition `json:"accessUserConditions,omitempty"`
	// BucketConditions contains a copy of the claim's underlying bucket conditions.
	BucketConditions []Condition `json:"bucketConditions,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XObjectBucket represents the internal composite of this claim
type XObjectBucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XObjectBucketSpec  `json:"spec"`
	Status ObjectBucketStatus `json:"status,omitempty"`
}

// XObjectBucketSpec defines the desired state of a ObjectBucket.
type XObjectBucketSpec struct {
	Parameters ObjectBucketParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef NamespacedName `json:"writeConnectionSecretToRef,omitempty"`
}

// NamespacedName describes an object reference by its name and namespace
type NamespacedName struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}
