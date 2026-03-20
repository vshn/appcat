package v1

import (
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshngarages.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshngarages.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshngarages.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshngarages.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.security.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshngarages.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.security.properties.allowAllNamespaces.default=true)"

// +kubebuilder:object:root=true

// VSHNGarage is the API for creating Garage instances.
type VSHNGarage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNGarage.
	Spec VSHNGarageSpec `json:"spec"`

	// Status reflects the observed state of a VSHNGarage.
	Status VSHNGarageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type VSHNGarageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNGarage `json:"items,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="self.parameters.instances == oldSelf.parameters.instances",message="Scaling after creation is not supported"

// VSHNGarageSpec defines the desired state of a VSHNGarage.
type VSHNGarageSpec struct {
	// Parameters are the configurable fields of a VSHNGarage.
	Parameters VSHNGarageParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

// VSHNGarageParameters are the configurable fields of a VSHNGarage.
type VSHNGarageParameters struct {
	// Service contains Garage DBaaS specific properties
	Service VSHNGarageServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNSizeSpec `json:"size,omitempty"`

	// Scheduling contains settings to control the scheduling of an instance.
	Scheduling VSHNDBaaSSchedulingSpec `json:"scheduling,omitempty"`

	// Security contains settings to control the security of a service.
	Security Security `json:"security,omitempty"`

	// Monitoring contains settings to control the monitoring of a service.
	Monitoring VSHNMonitoring `json:"monitoring,omitempty"`
	// +kubebuilder:default=1

	// Instances defines the number of instances to run.
	Instances int `json:"instances,omitempty"`
}

// VSHNGarageServiceSpec contains Garage DBaaS specific properties
type VSHNGarageServiceSpec struct {
	// +kubebuilder:validation:Enum="2"
	// +kubebuilder:default="2"

	// Version contains supported version of Garage.
	// Multiple versions are supported. The latest version 2 is the default version.
	Version string `json:"version,omitempty"`

	// +kubebuilder:default="5Gi"

	// MetadataStorage is the storage allocated for the metadata database.
	// If the instance is expected to handle millions of objects, this will need to be increased.
	MetadataStorage string `json:"metadataStorage,omitempty"`

	// +kubebuilder:validation:Enum="besteffort";"guaranteed"
	// +kubebuilder:default="besteffort"

	// ServiceLevel defines the service level of this service. Either Best Effort or Guaranteed Availability is allowed.
	ServiceLevel VSHNDBaaSServiceLevel `json:"serviceLevel,omitempty"`
}

// VSHNGarageStatus reflects the observed state of a VSHNGarage.
type VSHNGarageStatus struct {
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
	// Schedules keeps track of random generated schedules, is overwriten by
	// schedules set in the service's spec.
	Schedules VSHNScheduleStatus `json:"schedules,omitempty"`
}

func (v *VSHNGarage) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

func (v *VSHNGarage) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-garage-%s", v.GetName())
}

func (v *VSHNGarage) SetInstanceNamespaceStatus() {
	v.Status.InstanceNamespace = v.GetInstanceNamespace()
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNGarage represents the internal composite of this claim
type XVSHNGarage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XVSHNGarageSpec   `json:"spec"`
	Status XVSHNGarageStatus `json:"status,omitempty"`
}

// XVSHNGarageSpec defines the desired state of a VSHNGarage.
type XVSHNGarageSpec struct {
	// Parameters are the configurable fields of a VSHNGarage.
	Parameters VSHNGarageParameters `json:"parameters,omitempty"`

	xpv1.ResourceSpec `json:",inline"`
}

type XVSHNGarageStatus struct {
	VSHNGarageStatus    `json:",inline"`
	xpv1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNGarageList represents a list of composites
type XVSHNGarageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XVSHNGarage `json:"items"`
} // GetServiceName returns the name of this service
func (v *VSHNGarage) GetServiceName() string {
	return "garage"
}

// GetPDBLabels returns the labels to be used for the PodDisruptionBudget
// it should match one unique label od pod running in instanceNamespace
// without this, the PDB will match all pods
func (v *VSHNGarage) GetPDBLabels() map[string]string {
	return map[string]string{"app.kubernetes.io/name": "garage"}
}

// GetAllowAllNamespaces returns the AllowAllNamespaces field of this service
func (v *VSHNGarage) GetAllowAllNamespaces() bool {
	return v.Spec.Parameters.Security.AllowAllNamespaces
}

// GetAllowedNamespaces returns the AllowedNamespaces array of this service
func (v *VSHNGarage) GetAllowedNamespaces() []string {
	if v.Spec.Parameters.Security.AllowedNamespaces == nil {
		v.Spec.Parameters.Security.AllowedNamespaces = []string{}
	}
	return append(v.Spec.Parameters.Security.AllowedNamespaces, v.GetClaimNamespace())
}

func (v *VSHNGarage) GetSecurity() *Security {
	return &v.Spec.Parameters.Security
}

func (v *VSHNGarage) GetSize() VSHNSizeSpec {
	return v.Spec.Parameters.Size
}

func (v *VSHNGarage) GetMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

func (v *VSHNGarage) GetInstances() int {
	return v.Spec.Parameters.Instances
}

func (v *VSHNGarage) GetBillingName() string {
	return "appcat-" + v.GetServiceName()
}

func (v *VSHNGarage) GetClaimName() string {
	return v.GetLabels()["crossplane.io/claim-name"]
}

func (v *VSHNGarage) GetSLA() string {
	return string(v.Spec.Parameters.Service.ServiceLevel)
}

func (v *VSHNGarage) GetWorkloadName() string {
	return "garage"
}

func (v *VSHNGarage) GetWorkloadPodTemplateLabelsManager() PodTemplateLabelsManager {
	return &StatefulSetManager{}
}

// GetBackupRetention returns the retention definition for this backup.
func (v *VSHNGarage) GetBackupRetention() K8upRetentionPolicy {
	return K8upRetentionPolicy{}
}

// GetBackupSchedule returns the current backup schedule
func (v *VSHNGarage) GetBackupSchedule() string {
	return v.Status.Schedules.Backup
}

// GetFullMaintenanceSchedule returns
func (v *VSHNGarage) GetFullMaintenanceSchedule() VSHNDBaaSMaintenanceScheduleSpec {
	return VSHNDBaaSMaintenanceScheduleSpec{}
}

// IsBackupEnabled returns true if backups are enabled for this instance
func (v *VSHNGarage) IsBackupEnabled() bool {
	return false
}

func (v *VSHNGarage) GetUnmanagedBucket() *UnmanagedBucket {
	return nil
}
