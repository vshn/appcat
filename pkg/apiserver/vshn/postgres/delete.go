package postgres

import (
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.GracefulDeleter = &vshnPostgresBackupStorage{}
var _ rest.CollectionDeleter = &vshnPostgresBackupStorage{}
