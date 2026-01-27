package backup

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	cnpgv1 "github.com/vshn/appcat/v4/apis/cnpg/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CNPGBackupRunner handles creating and waiting for CNPG Backup resources
type CNPGBackupRunner struct {
	BaseRunner
}

// NewCNPGBackupRunner creates a new CNPGBackupRunner
func NewCNPGBackupRunner(c client.WithWatch, log logr.Logger) *CNPGBackupRunner {
	return &CNPGBackupRunner{
		BaseRunner: NewBaseRunner(c, log),
	}
}

// RunBackup finds the CNPG Cluster in the namespace and creates a one-off Backup resource
func (c *CNPGBackupRunner) RunBackup(ctx context.Context, namespace, backupName string) error {
	c.log.Info("Finding CNPG Cluster to create backup", "namespace", namespace)

	// List all Clusters in the namespace
	clusterList := &cnpgv1.ClusterList{}
	if err := c.k8sClient.List(ctx, clusterList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("failed to list CNPG Clusters: %w", err)
	}

	if len(clusterList.Items) == 0 {
		c.log.Info("No CNPG Cluster found in namespace, skipping backup", "namespace", namespace)
		return nil
	}

	// Use the first cluster found (there should typically only be one per instance)
	cluster := &clusterList.Items[0]
	c.log.Info("Found CNPG Cluster", "name", cluster.Name, "namespace", namespace)

	// Check if cluster is suspended (instances == 0)
	if IsClusterSuspended(cluster.Spec.Instances, c.log, namespace) {
		return nil
	}

	// Create a one-off Backup using typed API
	c.log.Info("Creating pre-maintenance backup", "namespace", namespace, "name", backupName)

	backup := &cnpgv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: namespace,
			Labels: map[string]string{
				"appcat.vshn.io/backup-type": "pre-maintenance",
			},
		},
		Spec: cnpgv1.BackupSpec{
			Cluster: cnpgv1.BackupCluster{
				Name: cluster.Name,
			},
			Method: cnpgv1.BackupMethodPlugin,
			PluginConfiguration: cnpgv1.PluginConfiguration{
				Name: "barman-cloud.cloudnative-pg.io",
			},
		},
	}

	// Create the backup resource
	if err := client.IgnoreAlreadyExists(c.k8sClient.Create(ctx, backup)); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	c.log.Info("Backup resource created or already exists, waiting for completion", "namespace", namespace, "name", backupName)

	// Wait for the backup to complete using watch
	backupToWatch := &cnpgv1.Backup{}
	backupToWatch.SetName(backupName)
	backupToWatch.SetNamespace(namespace)

	backupList := &cnpgv1.BackupList{}

	return WatchUntilDone(ctx, c.k8sClient, backupToWatch, backupList, c.timeout, c.checkDone, c.checkSuccess, c.log)
}

// checkDone returns true if the CNPG backup has reached a terminal state
func (c *CNPGBackupRunner) checkDone(obj client.Object) bool {
	backup, ok := TypeAssertObject[*cnpgv1.Backup](obj, "cnpgv1.Backup", c.log)
	if !ok {
		return false
	}

	// CNPG backup phases: pending, running, completed, failed
	return backup.Status.Phase == cnpgv1.BackupPhaseCompleted || backup.Status.Phase == cnpgv1.BackupPhaseFailed
}

// checkSuccess verifies the CNPG backup completed successfully
func (c *CNPGBackupRunner) checkSuccess(obj client.Object) error {
	backup, ok := TypeAssertObject[*cnpgv1.Backup](obj, "cnpgv1.Backup", c.log)
	if !ok {
		return fmt.Errorf("unexpected object type: expected cnpgv1.Backup, got %T", obj)
	}

	if backup.Status.Phase == cnpgv1.BackupPhaseCompleted {
		c.log.Info("Backup completed successfully",
			"namespace", backup.GetNamespace(),
			"name", backup.GetName())
		return nil
	}

	if backup.Status.Phase == cnpgv1.BackupPhaseFailed {
		// Try to get error information from status
		if backup.Status.Error != "" {
			return fmt.Errorf("backup failed: %s", backup.Status.Error)
		}
		return fmt.Errorf("backup failed with no detailed error information")
	}

	// Unknown phase
	return fmt.Errorf("backup finished with unknown phase: %s", backup.Status.Phase)
}
