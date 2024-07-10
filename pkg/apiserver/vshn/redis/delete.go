package redis

import (
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.GracefulDeleter = &vshnRedisBackupStorage{}
var _ rest.CollectionDeleter = &vshnRedisBackupStorage{}
