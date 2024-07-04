package common

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// InfoGetter will return various information about the given AppCat composite.
type InfoGetter interface {
	GetBackupSchedule() string
	GetBackupRetention() vshnv1.K8upRetentionPolicy
	GetServiceName() string
	GetLabels() map[string]string
	GetSize() vshnv1.VSHNSizeSpec
	GetInstances() int
	GetFullMaintenanceSchedule() vshnv1.VSHNDBaaSMaintenanceScheduleSpec
	GetMonitoring() vshnv1.VSHNMonitoring
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

// Composite can get and set the relevant information on a given composite.
type Composite interface {
	InfoGetter
	client.Object
	SetInstanceNamespaceStatus()
	AllowedNamespaceGetter
}

type AllowedNamespaceGetter interface {
	GetAllowAllNamespaces() bool
	GetAllowedNamespaces() []string
}
