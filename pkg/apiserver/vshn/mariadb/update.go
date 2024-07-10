package mariadb

import (
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Updater = &vshnMariaDBBackupStorage{}
var _ rest.CreaterUpdater = &vshnMariaDBBackupStorage{}
