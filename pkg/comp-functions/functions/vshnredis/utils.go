package vshnredis

import (
	"encoding/base64"
	"fmt"

	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
)

func getInstanceNamespace(comp *vshnv1.VSHNRedis) string {
	return fmt.Sprintf("vshn-redis-%s", comp.GetName())
}

func decodeBase64(encoded []byte) (string, error) {
	tmp := make([]byte, len(encoded))
	_, err := base64.StdEncoding.Decode(tmp, encoded)
	if err != nil {
		return "", err
	}
	return string(tmp), nil
}
