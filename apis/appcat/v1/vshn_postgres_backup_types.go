package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
)

// +kubebuilder:rbac:groups="stackgres.io",resources=sgbackups,verbs=get;list;watch
// +kubebuilder:rbac:groups="vshn.appcat.vshn.io",resources=vshnpostgresqls,verbs=get;list;watch

var (
	// ResourceBackup is the name of this backup resource in plural form
	ResourceBackup = "vshnpostgresbackups"

	// Metadata holds field path name metadata
	Metadata = "metadata"
	// Status holds field path name status
	Status = "status"
	// Process holds field path name process
	Process = "process"
	// BackupInformation holds field path name backupInformation
	BackupInformation = "backupInformation"
	// DatabaseInstance holds field path name databaseInstance
	DatabaseInstance = "databaseInstance"
	// InstanceNamespace holds field path name instanceNamespace
	InstanceNamespace = "instanceNamespace"
)

// VSHNPostgreSQLName represents the name of a VSHNPostgreSQL
type VSHNPostgreSQLName string

// VSHNPostgreSQLNamespace represents the namespace of a VSHNPostgreSQL
type VSHNPostgreSQLNamespace string

// +kubebuilder:object:root=true

// VSHNPostgresBackup defines VSHN managed PostgreSQL backups
type VSHNPostgresBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Status holds the backup specific metadata.
	Status VSHNPostgresBackupStatus `json:"status,omitempty"`
}

// VSHNPostgresBackupStatus defines the desired state of VSHNPostgresBackup
type VSHNPostgresBackupStatus struct {
	// Process holds status information of the backup process
	Process runtime.RawExtension `json:"process,omitempty"`
	// BackupInformation holds specific backup information
	BackupInformation runtime.RawExtension `json:"backupInformation,omitempty"`
	// DatabaseInstance is the database from which the backup has been done
	DatabaseInstance string `json:"databaseInstance"`
}

// VSHNPostgresBackup needs to implement the builder resource interface
var _ resource.Object = &VSHNPostgresBackup{}

func (in *VSHNPostgresBackup) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *VSHNPostgresBackup) NamespaceScoped() bool {
	return true
}

func (in *VSHNPostgresBackup) New() runtime.Object {
	return &VSHNPostgresBackup{}
}

func (in *VSHNPostgresBackup) NewList() runtime.Object {
	return &VSHNPostgresBackupList{}
}

// GetGroupVersionResource returns the GroupVersionResource for this resource.
// The resource should be the all lowercase and pluralized kind
func (in *VSHNPostgresBackup) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    GroupVersion.Group,
		Version:  GroupVersion.Version,
		Resource: "vshnpostgresbackups",
	}
}

// IsStorageVersion returns true if the object is also the internal version -- i.e. is the type defined for the API group or an alias to this object.
// If false, the resource is expected to implement MultiVersionObject interface.
func (in *VSHNPostgresBackup) IsStorageVersion() bool {
	return true
}

// +kubebuilder:object:root=true

// VSHNPostgresBackupList defines a list of VSHNPostgresBackup
type VSHNPostgresBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNPostgresBackup `json:"items"`
}

var _ resource.ObjectList = &VSHNPostgresBackupList{}

func (in *VSHNPostgresBackupList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

func init() {
	SchemeBuilder.Register(&VSHNPostgresBackup{}, &VSHNPostgresBackupList{})
}

func New() *VSHNPostgresBackup {
	return &VSHNPostgresBackup{}
}

// SGBackupInfo holds necessary data for VSHNPostgresBackup
type SGBackupInfo struct {
	metav1.ObjectMeta
	Process           runtime.RawExtension
	BackupInformation runtime.RawExtension
}

// NewVSHNPostgresBackup creates a new VSHNPostgresBackup out of a SGBackupInfo and a database instance
func NewVSHNPostgresBackup(backup *SGBackupInfo, db, originalNamespace string) *VSHNPostgresBackup {
	if backup == nil || originalNamespace == "" || db == "" {
		return nil
	}

	vshnPostgresBackup := &VSHNPostgresBackup{
		ObjectMeta: backup.ObjectMeta,
		Status:     VSHNPostgresBackupStatus{},
	}

	vshnPostgresBackup.Status.DatabaseInstance = db
	vshnPostgresBackup.Namespace = originalNamespace

	if backup.Process.Object != nil {
		vshnPostgresBackup.Status.Process = backup.Process
	}

	if backup.BackupInformation.Object != nil {
		vshnPostgresBackup.Status.BackupInformation = backup.BackupInformation
	}
	return vshnPostgresBackup
}
