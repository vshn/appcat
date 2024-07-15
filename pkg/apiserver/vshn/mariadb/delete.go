package mariadb

import (
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.GracefulDeleter = &vshnMariaDBBackupStorage{}
var _ rest.CollectionDeleter = &vshnMariaDBBackupStorage{}
