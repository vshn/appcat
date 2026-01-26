package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

// Backup is the API for creating one-off backups.
// This is a partial reconstruction containing only fields we need for maintenance operations.
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a Backup.
	Spec BackupSpec `json:"spec"`

	// Status defines the status of a Backup.
	Status BackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupList contains a list of Backup resources.
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Backup `json:"items"`
}

// BackupSpec defines the desired state of a Backup.
type BackupSpec struct {
	// Cluster is the reference to the PostgreSQL cluster to backup
	Cluster BackupCluster `json:"cluster"`

	// Method defines the backup method. Can be "barmanObjectStore" or "plugin"
	Method BackupMethod `json:"method,omitempty"`

	// Configuration parameters passed to the plugin managing this backup
	PluginConfiguration `json:"pluginConfiguration,omitempty"`
}

// BackupCluster defines the cluster reference for a backup.
type BackupCluster struct {
	// Name of the cluster to backup
	Name string `json:"name"`
}

// BackupMethod defines the backup method
// +kubebuilder:validation:Enum=barmanObjectStore;plugin;volumeSnapshot
type BackupMethod string

const (
	BackupMethodBarmanObjectStore BackupMethod = "barmanObjectStore"
	BackupMethodPlugin            BackupMethod = "plugin"
	BackupMethodVolumeSnapshot    BackupMethod = "volumeSnapshot"
)

type PluginConfiguration struct {
	// Name is the name of the plugin managing this backup
	Name string `json:"name"`
	// Parameters are the configuration parameters passed to the backup plugin for this backup
	Parameters map[string]string `json:"parameters,omitempty"`
}

// BackupStatus defines the observed state of a Backup.
type BackupStatus struct {
	// Phase describes the current phase of the backup
	// +kubebuilder:validation:Enum=pending;running;completed;failed
	Phase BackupPhase `json:"phase,omitempty"`

	// Error contains error details if the backup failed
	Error string `json:"error,omitempty"`

	// StartedAt is the time when the backup was started
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// StoppedAt is the time when the backup was stopped
	StoppedAt *metav1.Time `json:"stoppedAt,omitempty"`

	// BackupID is the backup identifier
	BackupID string `json:"backupId,omitempty"`

	// BackupName is the backup name in the object store
	BackupName string `json:"backupName,omitempty"`

	// DestinationPath is the path where the backup is stored
	DestinationPath string `json:"destinationPath,omitempty"`
}

// BackupPhase defines the phase of the backup
type BackupPhase string

const (
	BackupPhasePending   BackupPhase = "pending"
	BackupPhaseRunning   BackupPhase = "running"
	BackupPhaseCompleted BackupPhase = "completed"
	BackupPhaseFailed    BackupPhase = "failed"
)
