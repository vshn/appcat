// +kubebuilder:object:generate=true
// +groupName=vshn.appcat.vshn.io
// +versionName=v1

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "syn.tools", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(
		&CompositeRedisInstance{},
		&CompositeRedisInstanceList{},
		&CompositeMariaDBInstance{},
		&CompositeMariaDBInstanceList{},
		&CompositeMariaDBDatabaseInstance{},
		&CompositeMariaDBDatabaseInstanceList{},
		&CompositeMariaDBUserInstance{},
		&CompositeMariaDBUserInstanceList{},
	)
}
