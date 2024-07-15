package redis

import (
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Updater = &vshnRedisBackupStorage{}
var _ rest.CreaterUpdater = &vshnRedisBackupStorage{}
