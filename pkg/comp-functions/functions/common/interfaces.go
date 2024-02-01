package common

import vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"

// InfoGetter will return various information about the given AppCat composite.
type InfoGetter interface {
	GetName() string
	GetInstanceNamespace() string
	GetBackupSchedule() string
	GetBackupRetention() vshnv1.K8upRetentionPolicy
	GetServiceName() string
	GetInstanceNamespaceRegex() (string, []string, error)
}
