package vshnminio

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

func getInstanceNamespace(comp *vshnv1.VSHNMinio) string {
	return "vshn-minio-" + comp.GetName()
}
