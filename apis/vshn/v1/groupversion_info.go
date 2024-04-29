// +kubebuilder:object:generate=true
// +groupName=vshn.appcat.vshn.io
// +versionName=v1

package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "vshn.appcat.vshn.io", Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(
		&VSHNPostgreSQL{},
		&VSHNPostgreSQLList{},
		&XVSHNPostgreSQL{},
		&XVSHNPostgreSQLList{},

		&VSHNRedis{},
		&VSHNRedisList{},
		&XVSHNRedis{},
		&XVSHNRedisList{},

		&VSHNMinio{},
		&VSHNMinioList{},
		&XVSHNMinio{},
		&XVSHNMinioList{},

		&XVSHNKeycloak{},
		&XVSHNKeycloakList{},
		&VSHNKeycloakList{},
		&VSHNKeycloak{},
		&XVSHNMariaDB{},
		&XVSHNMariaDBList{},
		&VSHNMariaDB{},
		&VSHNMariaDBList{},
	)
}
