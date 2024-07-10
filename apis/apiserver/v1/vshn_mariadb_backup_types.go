package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
)

// VSHNMariaDBBackup needs to implement the builder resource interface
var _ resource.Object = &VSHNMariaDBBackup{}
var _ resource.ObjectList = &VSHNMariaDBBackupList{}

// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
type VSHNMariaDBBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status VSHNMariaDBBackupStatus `json:"status,omitempty"`
}

// +k8s:openapi-gen=true
type VSHNMariaDBBackupStatus struct {
	ID       string      `json:"id,omitempty"`
	Date     metav1.Time `json:"date,omitempty"`
	Instance string      `json:"instance,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:openapi-gen=true
type VSHNMariaDBBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNMariaDBBackup `json:"items,omitempty"`
}

// GetGroupVersionResource returns the GroupVersionResource for this resource.
// The resource should be the all lowercase and pluralized kind
func (in *VSHNMariaDBBackup) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    GroupVersion.Group,
		Version:  GroupVersion.Version,
		Resource: "vshnmariadbbackups",
	}
}

func (in *VSHNMariaDBBackup) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

// IsStorageVersion returns true if the object is also the internal version -- i.e. is the type defined for the API group or an alias to this object.
// If false, the resource is expected to implement MultiVersionObject interface.
func (in *VSHNMariaDBBackup) IsStorageVersion() bool {
	return true
}

func (in *VSHNMariaDBBackup) NamespaceScoped() bool {
	return true
}

func (in *VSHNMariaDBBackup) New() runtime.Object {
	return &VSHNMariaDBBackup{}
}

func (in *VSHNMariaDBBackup) NewList() runtime.Object {
	return &VSHNMariaDBBackupList{}
}

func (in *VSHNMariaDBBackupList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}
