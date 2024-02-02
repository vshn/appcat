package common

import vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"

// InfoGetter will return various information about the given AppCat composite.
type InfoGetter interface {
	GetBackupSchedule() string
	GetBackupRetention() vshnv1.K8upRetentionPolicy
	GetServiceName() string
	GetLabels() map[string]string
	InstanceNamespaceInfo
}

// InstanceNamespaceInfo provides all the necessary information to create
// an instance namespace.
type InstanceNamespaceInfo interface {
	GetName() string
	GetClaimNamespace() string
	GetInstanceNamespace() string
	GetLabels() map[string]string
}
