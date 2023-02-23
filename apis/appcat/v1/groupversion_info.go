// Package v1 contains API Schema definitions for the appcat-server v1 API group
// +kubebuilder:object:generate=true
// +kubebuilder:skip
// +groupName=api.appcat.vshn.io
package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "api.appcat.vshn.io", Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func GetGroupResource(resource string) schema.GroupResource {
	return schema.GroupResource{Group: GroupVersion.Group, Resource: resource}
}
