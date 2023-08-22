package vshnredis

import (
	"fmt"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

func getInstanceNamespace(comp *vshnv1.VSHNRedis) string {
	return fmt.Sprintf("vshn-redis-%s", comp.GetName())
}
