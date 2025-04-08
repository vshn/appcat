package v1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	crossplane "github.com/crossplane/crossplane/apis/apiextensions/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
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

//go:generate yq -i e ../generated/appcat.vshn.io_objectbuckets.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.security.default={})"
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
	WriteConnectionSecretToRef vshnv1.LocalObjectReference     `json:"writeConnectionSecretToRef,omitempty"`
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

	// Region is the name of the region where the bucket shall be created.
	// The region must be available in the S3 endpoint.
	Region string `json:"region,omitempty"`

	// +kubebuilder:default=DeleteAll

	// BucketDeletionPolicy determines how buckets should be deleted when Bucket is deleted.
	//  `DeleteIfEmpty` only deletes the bucket if the bucket is empty.
	//  `DeleteAll` recursively deletes all objects in the bucket and then removes it.
	BucketDeletionPolicy BucketDeletionPolicy `json:"bucketDeletionPolicy,omitempty"`

	// Security defines the security of a service
	Security vshnv1.Security `json:"security,omitempty"`
}

// ObjectBucketStatus reflects the observed state of a ObjectBucket.
type ObjectBucketStatus struct {
	// AccessUserConditions contains a copy of the claim's underlying user account conditions.
	AccessUserConditions []vshnv1.Condition `json:"accessUserConditions,omitempty"`
	// BucketConditions contains a copy of the claim's underlying bucket conditions.
	BucketConditions []vshnv1.Condition `json:"bucketConditions,omitempty"`

	xpv1.ResourceStatus `json:",inline"`
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

	xpv1.ResourceSpec `json:",inline"`
}

// NamespacedName describes an object reference by its name and namespace
type NamespacedName struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

func (v *ObjectBucket) GetSecurity() *vshnv1.Security {
	return &v.Spec.Parameters.Security
}
