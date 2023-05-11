package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
	WriteConnectionSecretToRef LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
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
}

// ObjectBucketStatus reflects the observed state of a ObjectBucket.
type ObjectBucketStatus struct {
	// AccessUserConditions contains a copy of the claim's underlying user account conditions.
	AccessUserConditions []Condition `json:"accessUserConditions,omitempty"`
	// BucketConditions contains a copy of the claim's underlying bucket conditions.
	BucketConditions []Condition `json:"bucketConditions,omitempty"`
}
