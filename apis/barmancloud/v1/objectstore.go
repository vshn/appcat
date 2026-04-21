package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

// ObjectStore is the API for creating barman cloud object stores.
// This is a partial reconstruction containing only fields we need for CNPG restore operations.
type ObjectStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of an ObjectStore.
	Spec ObjectStoreSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// ObjectStoreList contains a list of ObjectStore resources.
type ObjectStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ObjectStore `json:"items"`
}

// ObjectStoreSpec defines the desired state of an ObjectStore.
type ObjectStoreSpec struct {
	// Configuration defines the barman cloud backup configuration.
	Configuration ObjectStoreConfiguration `json:"configuration"`
}

// ObjectStoreConfiguration defines the backup storage configuration.
type ObjectStoreConfiguration struct {
	// EndpointURL is the S3-compatible endpoint URL.
	EndpointURL string `json:"endpointURL,omitempty"`

	// DestinationPath is the object store destination path (e.g. "s3://bucket/").
	DestinationPath string `json:"destinationPath"`

	// S3Credentials defines the S3 credentials for accessing the object store.
	S3Credentials *S3Credentials `json:"s3Credentials,omitempty"`
}

// S3Credentials defines references to secrets containing S3 credentials.
type S3Credentials struct {
	// AccessKeyId is a reference to a secret containing the access key ID.
	AccessKeyId SecretKeySelector `json:"accessKeyId"`

	// SecretAccessKey is a reference to a secret containing the secret access key.
	SecretAccessKey SecretKeySelector `json:"secretAccessKey"`

	// Region is a reference to a secret containing the AWS region.
	Region SecretKeySelector `json:"region"`
}

// SecretKeySelector references a specific key within a Secret.
type SecretKeySelector struct {
	// Name of the secret.
	Name string `json:"name"`

	// Key within the secret.
	Key string `json:"key"`
}
