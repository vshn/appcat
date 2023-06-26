package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
)

// VSHNRedisBackup needs to implement the builder resource interface
var _ resource.Object = &VSHNRedisBackup{}
var _ resource.ObjectList = &VSHNRedisBackupList{}

// +kubebuilder:object:root=true

type VSHNRedisBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status VSHNRedisBackupStatus `json:"status,omitempty"`
}

type VSHNRedisBackupStatus struct {
	ID       string      `json:"id,omitempty"`
	Date     metav1.Time `json:"date,omitempty"`
	Instance string      `json:"instance,omitempty"`
}

// +kubebuilder:object:root=true

type VSHNRedisBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNRedisBackup `json:"items,omitempty"`
}

// GetGroupVersionResource returns the GroupVersionResource for this resource.
// The resource should be the all lowercase and pluralized kind
func (in *VSHNRedisBackup) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    GroupVersion.Group,
		Version:  GroupVersion.Version,
		Resource: "vshnredisbackups",
	}
}

func (in *VSHNRedisBackup) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

// IsStorageVersion returns true if the object is also the internal version -- i.e. is the type defined for the API group or an alias to this object.
// If false, the resource is expected to implement MultiVersionObject interface.
func (in *VSHNRedisBackup) IsStorageVersion() bool {
	return true
}

func (in *VSHNRedisBackup) NamespaceScoped() bool {
	return true
}

func (in *VSHNRedisBackup) New() runtime.Object {
	return &VSHNRedisBackup{}
}

func (in *VSHNRedisBackup) NewList() runtime.Object {
	return &VSHNRedisBackupList{}
}

func (in *VSHNRedisBackupList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}
