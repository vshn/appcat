package v1

import (
	"fmt"

	v1 "github.com/vshn/appcat/v4/apis/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnminios.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnminios.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnminios.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.default={})"

// +kubebuilder:object:root=true

// VSHNMinio is the API for creating Minio instances.
type VSHNMinio struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNMinio.
	Spec VSHNMinioSpec `json:"spec"`

	// Status reflects the observed state of a VSHNMinio.
	Status VSHNMinioStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type VSHNMinioList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNMinio `json:"items,omitempty"`
}

// VSHNMinioSpec defines the desired state of a VSHNMinio.
type VSHNMinioSpec struct {
	// Parameters are the configurable fields of a VSHNMinio.
	Parameters VSHNMinioParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef v1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

// VSHNMinioParameters are the configurable fields of a VSHNMinio.
type VSHNMinioParameters struct {
	// Size contains settings to control the sizing of a service.
	Size VSHNSizeSpec `json:"size,omitempty"`

	// Backup contains settings to control how the instance should get backed up.
	Backup K8upBackupSpec `json:"backup,omitempty"`

	// Restore contains settings to control the restore of an instance.
	Restore K8upRestoreSpec `json:"restore,omitempty"`

	// +kubebuilder:default=4
	// +kubebuilder:validation:Minimum=4

	// Instances configures the number of Minio instances for the cluster.
	// Each instance contains one Minio server.
	Instances int `json:"instances,omitempty"`

	// StorageClass configures the storageClass to use for the PVC used by MinIO.
	StorageClass string `json:"storageClass,omitempty"`

	// Service contains the Minio specific configurations
	Service VSHNMinioServiceSpec `json:"service,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance VSHNDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`
}

// VSHNMinioServiceSpec contains Redis DBaaS specific properties
type VSHNMinioServiceSpec struct {

	// +kubebuilder:default="distributed"
	// +kubebuilder:validation:Enum=distributed;standalone

	// Mode configures the mode of MinIO.
	// Valid values are "distributed" and "standalone".
	Mode string `json:"mode,omitempty"`
}

// VSHNMinioStatus reflects the observed state of a VSHNMinio.
type VSHNMinioStatus struct {
	// MinioConditions contains the status conditions of the backing object.
	NamespaceConditions []v1.Condition `json:"namespaceConditions,omitempty"`
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
}

func (v *VSHNMinio) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

func (v *VSHNMinio) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-minio-%s", v.GetName())

}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNMinios represents the internal composite of this claim
type XVSHNMinio VSHNMinio

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNMiniosList represents a list of composites
type XVSHNMinioList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XVSHNMinio `json:"items"`
}