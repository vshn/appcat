package vshnredis

import (
	"fmt"
	"strings"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

func getInstanceNamespace(comp *vshnv1.VSHNRedis) string {
	return fmt.Sprintf("vshn-redis-%s", comp.GetName())
}

// parseImageRef splits a full image reference into registry, repository and tag.
func parseImageRef(ref string) (registry, repository, tag string) {
	repository = ref

	if i := strings.LastIndex(repository, ":"); i > strings.LastIndex(repository, "/") {
		tag = repository[i+1:]
		repository = repository[:i]
	}

	if i := strings.Index(repository, "/"); i > 0 {
		first := repository[:i]
		if strings.ContainsAny(first, ".:") || first == "localhost" {
			registry = first
			repository = repository[i+1:]
		}
	}

	return registry, repository, tag
}
