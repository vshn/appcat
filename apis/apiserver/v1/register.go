// Package v1 contains API Schema definitions for the appcat-server v1 API group
// +kubebuilder:object:generate=true
// +kubebuilder:skip
// +groupName=api.appcat.vshn.io
package v1

//go:generate go run k8s.io/kube-openapi/cmd/openapi-gen . k8s.io/apimachinery/pkg/api/resource k8s.io/apimachinery/pkg/apis/meta/v1 k8s.io/apimachinery/pkg/runtime k8s.io/apimachinery/pkg/version --output-dir ../../../pkg/openapi --output-pkg openapi

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "api.appcat.vshn.io", Version: "v1"}

	SchemeBuilder      runtime.SchemeBuilder
	localSchemeBuilder = &SchemeBuilder
	AddToScheme        = localSchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(addKnownTypes)
}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&AppCat{},
		&AppCatList{},
		&VSHNPostgresBackup{},
		&VSHNPostgresBackupList{},
		&VSHNRedisBackup{},
		&VSHNRedisBackupList{},
		&VSHNMariaDBBackup{},
		&VSHNMariaDBBackupList{},
		&VSHNNextcloudBackup{},
		&VSHNNextcloudBackupList{},
		&VSHNKeycloakBackup{},
		&VSHNKeycloakBackupList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}

func GetGroupResource(resource string) schema.GroupResource {
	return schema.GroupResource{Group: GroupVersion.Group, Resource: resource}
}
