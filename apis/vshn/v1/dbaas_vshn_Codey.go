package v1

import (
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_codeys.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_codeys.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_codeys.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.service.default={})"

// +kubebuilder:object:root=true

// Codey is the API for creating Codey instances.
type Codey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a Codey.
	Spec CodeySpec `json:"spec"`

	// Status reflects the observed state of a Codey.
	Status CodeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type CodeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Codey `json:"items,omitempty"`
}

// CodeySpec defines the desired state of a Codey.
type CodeySpec struct {
	// Parameters are the configurable fields of a Codey.
	Parameters CodeyParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

// CodeyParameters are the configurable fields of a Codey.
type CodeyParameters struct {
	// Service contains Codey DBaaS specific properties
	Service CodeyServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNSizeSpec `json:"size,omitempty"`
}

// CodeyServiceSpec contains Codey DBaaS specific properties
type CodeyServiceSpec struct {

	// Version contains supported version of Codey.
	// Multiple versions are supported.
	MajorVersion string `json:"majorVersion,omitempty"`

	// Codeysettings contains additional Codey settings.
	AdminEmail string `json:"adminEmail,omitempty"`
}

// CodeySizeSpec contains settings to control the sizing of a service.
type CodeySizeSpec struct {

	// CPURequests defines the requests amount of Kubernetes CPUs for an instance.
	CPURequests string `json:"cpuRequests,omitempty"`

	// CPULimits defines the limits amount of Kubernetes CPUs for an instance.
	CPULimits string `json:"cpuLimits,omitempty"`

	// MemoryRequests defines the requests amount of memory in units of bytes for an instance.
	MemoryRequests string `json:"memoryRequests,omitempty"`

	// MemoryLimits defines the limits amount of memory in units of bytes for an instance.
	MemoryLimits string `json:"memoryLimits,omitempty"`

	// Disk defines the amount of disk space for an instance.
	Disk string `json:"disk,omitempty"`

	// Plan is the name of the resource plan that defines the compute resources.
	Plan string `json:"plan,omitempty"`
}

// CodeyStatus reflects the observed state of a Codey.
type CodeyStatus struct {
	NamespaceConditions         []Condition `json:"namespaceConditions,omitempty"`
	SelfSignedIssuerConditions  []Condition `json:"selfSignedIssuerConditions,omitempty"`
	LocalCAConditions           []Condition `json:"localCAConditions,omitempty"`
	CaCertificateConditions     []Condition `json:"caCertificateConditions,omitempty"`
	ServerCertificateConditions []Condition `json:"serverCertificateConditions,omitempty"`
	ClientCertificateConditions []Condition `json:"clientCertificateConditions,omitempty"`
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
	// Schedules keeps track of random generated schedules, is overwriten by
	// schedules set in the service's spec.
	Schedules VSHNScheduleStatus `json:"schedules,omitempty"`
}

func (v *Codey) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

func (v *Codey) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-codey-%s", v.GetName())
}

func (v *Codey) SetInstanceNamespaceStatus() {
	v.Status.InstanceNamespace = v.GetInstanceNamespace()
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XCodey represents the internal composite of this claim
type XCodey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XCodeySpec   `json:"spec"`
	Status XCodeyStatus `json:"status,omitempty"`
}

// XCodeySpec defines the desired state of a Codey.
type XCodeySpec struct {
	// Parameters are the configurable fields of a Codey.
	Parameters CodeyParameters `json:"parameters,omitempty"`

	xpv1.ResourceSpec `json:",inline"`
}

type XCodeyStatus struct {
	CodeyStatus         `json:",inline"`
	xpv1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XCodeyList represents a list of composites
type XCodeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XCodey `json:"items"`
} // GetServiceName returns the name of this service
func (v *Codey) GetServiceName() string {
	return "codey"
}

// GetPDBLabels returns the labels to be used for the PodDisruptionBudget
// it should match one unique label od pod running in instanceNamespace
// without this, the PDB will match all pods
func (v *Codey) GetPDBLabels() map[string]string {
	return map[string]string{}
}

func (v *Codey) GetSize() VSHNSizeSpec {
	return v.Spec.Parameters.Size
}

func (v *Codey) GetBillingName() string {
	return "appcat-" + v.GetServiceName()
}

func (v *Codey) GetClaimName() string {
	return v.GetLabels()["crossplane.io/claim-name"]
}

func (v *Codey) GetWorkloadName() string {
	return v.GetName() + "-codeydeployment"
}

func (v *Codey) GetWorkloadPodTemplateLabelsManager() PodTemplateLabelsManager {
	return &StatefulSetManager{}
}
