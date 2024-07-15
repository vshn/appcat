package appcat

import (
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.GracefulDeleter = &appcatStorage{}
var _ rest.CollectionDeleter = &appcatStorage{}
