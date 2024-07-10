package appcat

import (
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Updater = &appcatStorage{}
var _ rest.CreaterUpdater = &appcatStorage{}
