package common

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// InfoGetter will return various information about the given AppCat composite.
type InfoGetter interface {
	GetBackupSchedule() string
	GetBackupRetention() vshnv1.K8upRetentionPolicy
	IsBackupEnabled() bool
	GetServiceName() string
	GetLabels() map[string]string
	GetSize() vshnv1.VSHNSizeSpec
	InstancesGetter
	GetFullMaintenanceSchedule() vshnv1.VSHNDBaaSMaintenanceScheduleSpec
	GetMonitoring() vshnv1.VSHNMonitoring
	GetSecurity() *vshnv1.Security
	InstanceNamespaceInfo
	GetPDBLabels() map[string]string
	GetWorkloadPodTemplateLabelsManager() vshnv1.PodTemplateLabelsManager
	GetWorkloadName() string
	GetClaimName() string
	GetSLA() string
	GetBillingName() string
}

// InstanceNamespaceInfo provides all the necessary information to create
// an instance namespace.
type InstanceNamespaceInfo interface {
	InstanceNamespaceGetter
	GetName() string
	GetClaimNamespace() string
	GetLabels() map[string]string
}

// InstanceNamespaceGetter returns the instance namespace of the given object
type InstanceNamespaceGetter interface {
	GetInstanceNamespace() string
}

// InstancesGetter returns the number of instances of the given object
type InstancesGetter interface {
	GetInstances() int
}

// CompositionNameGetter returns the composition name of the given object
type CompositionNameGetter interface {
	GetCompositionName() string
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

// Required to get info required for alerting.
type Alerter interface {
	GetVSHNMonitoring() vshnv1.VSHNMonitoring
	GetInstanceNamespace() string
}
