package postgres

import (
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Updater = &vshnPostgresBackupStorage{}
var _ rest.CreaterUpdater = &vshnPostgresBackupStorage{}
