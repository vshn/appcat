package utils

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

// IsKindAvailable will check if the given kind is available
func IsKindAvailable(gv schema.GroupVersion, kind string, config *rest.Config) bool {
	d, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return false
	}
	resources, err := d.ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		return false
	}

	for _, res := range resources.APIResources {
		if res.Kind == kind {
			return true
		}
	}
	return false
}
