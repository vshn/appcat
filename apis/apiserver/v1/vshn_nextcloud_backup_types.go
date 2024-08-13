package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
)

// VSHNNextcloudBackup needs to implement the builder resource interface
var _ resource.Object = &VSHNNextcloudBackup{}
var _ resource.ObjectList = &VSHNNextcloudBackupList{}

// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
type VSHNNextcloudBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status VSHNNextcloudBackupStatus `json:"status,omitempty"`
}

// +k8s:openapi-gen=true
type VSHNNextcloudBackupStatus struct {
	// FileBackupAvailable indicates if this backup contains a file backup for Nextcloud.
	// Not every file backup might have a database backup associated, because the retention is not enforced at the same time.
	FileBackupAvailable bool
	// DatabaseBackupAvailable indicates if this backup contains a database backup for Nextcloud.
	// Not every file backup might have a database backup associated, because the retention is not enforced at the same time.
	DatabaseBackupAvailable bool
	NextcloudFileBackup     VSHNNextcloudFileBackupStatus `json:"nextcloudFileBackup,omitempty"`
	DatabaseBackupStatus    VSHNPostgresBackupStatus      `json:"databaseBackupStatus,omitempty"`
}

// +k8s:openapi-gen=true
type VSHNNextcloudFileBackupStatus struct {
	ID       string      `json:"id,omitempty"`
	Date     metav1.Time `json:"date,omitempty"`
	Instance string      `json:"instance,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
type VSHNNextcloudBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNNextcloudBackup `json:"items,omitempty"`
}

// GetGroupVersionResource returns the GroupVersionResource for this resource.
// The resource should be the all lowercase and pluralized kind
func (in *VSHNNextcloudBackup) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    GroupVersion.Group,
		Version:  GroupVersion.Version,
		Resource: "vshnnextcloudbackups",
	}
}

func (in *VSHNNextcloudBackup) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

// IsStorageVersion returns true if the object is also the internal version -- i.e. is the type defined for the API group or an alias to this object.
// If false, the resource is expected to implement MultiVersionObject interface.
func (in *VSHNNextcloudBackup) IsStorageVersion() bool {
	return true
}

func (in *VSHNNextcloudBackup) NamespaceScoped() bool {
	return true
}

func (in *VSHNNextcloudBackup) New() runtime.Object {
	return &VSHNNextcloudBackup{}
}

func (in *VSHNNextcloudBackup) NewList() runtime.Object {
	return &VSHNNextcloudBackupList{}
}

func (in *VSHNNextcloudBackupList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}
