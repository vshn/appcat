package garage

import "k8s.io/apimachinery/pkg/runtime/schema"

const (
	Group   = "garage.rajsingh.info"
	Version = "v1beta1"
)

var (
	GroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	GarageBucketGVK  = GroupVersion.WithKind("GarageBucket")
	GarageKeyGVK     = GroupVersion.WithKind("GarageKey")
	GarageClusterGVK = GroupVersion.WithKind("GarageCluster")
)
