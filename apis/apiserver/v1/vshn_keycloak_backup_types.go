package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
)

// VSHNKeycloakBackup needs to implement the builder resource interface
var _ resource.Object = &VSHNKeycloakBackup{}
var _ resource.ObjectList = &VSHNKeycloakBackupList{}

// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
type VSHNKeycloakBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status VSHNKeycloakBackupStatus `json:"status,omitempty"`
}

// +k8s:openapi-gen=true
type VSHNKeycloakBackupStatus struct {
	// DatabaseBackupAvailable indicates if this backup contains a database backup for Keycloak.
	// Not every file backup might have a database backup associated, because the retention is not enforced at the same time.
	DatabaseBackupAvailable bool
	DatabaseBackupStatus    VSHNPostgresBackupStatus `json:"databaseBackupStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
type VSHNKeycloakBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNKeycloakBackup `json:"items,omitempty"`
}

// GetGroupVersionResource returns the GroupVersionResource for this resource.
// The resource should be the all lowercase and pluralized kind
func (in *VSHNKeycloakBackup) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    GroupVersion.Group,
		Version:  GroupVersion.Version,
		Resource: "vshnkeycloakbackups",
	}
}

func (in *VSHNKeycloakBackup) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

// IsStorageVersion returns true if the object is also the internal version -- i.e. is the type defined for the API group or an alias to this object.
// If false, the resource is expected to implement MultiVersionObject interface.
func (in *VSHNKeycloakBackup) IsStorageVersion() bool {
	return true
}

func (in *VSHNKeycloakBackup) NamespaceScoped() bool {
	return true
}

func (in *VSHNKeycloakBackup) New() runtime.Object {
	return &VSHNKeycloakBackup{}
}

func (in *VSHNKeycloakBackup) NewList() runtime.Object {
	return &VSHNKeycloakBackupList{}
}

func (in *VSHNKeycloakBackupList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}
