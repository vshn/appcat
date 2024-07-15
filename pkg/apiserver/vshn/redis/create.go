package redis

import (
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Creater = &vshnRedisBackupStorage{}
