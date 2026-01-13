package backup

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	cnpgv1 "github.com/vshn/appcat/v4/apis/cnpg/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	// Create a one-off Backup using unstructured to avoid needing full CNPG API types
	c.log.Info("Creating pre-maintenance backup", "namespace", namespace, "name", backupName)

	backup := &unstructured.Unstructured{}
	backup.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "postgresql.cnpg.io",
		Version: "v1",
		Kind:    "Backup",
	})
	backup.SetName(backupName)
	backup.SetNamespace(namespace)
	backup.SetLabels(map[string]string{
		"appcat.vshn.io/backup-type": "pre-maintenance",
	})

	// Set the spec
	if err := unstructured.SetNestedMap(backup.Object, map[string]interface{}{
		"cluster": map[string]interface{}{
			"name": cluster.Name,
		},
		"method": "barmanObjectStore",
	}, "spec"); err != nil {
		return fmt.Errorf("failed to set backup spec: %w", err)
	}

	// Create the backup resource
	if err := client.IgnoreAlreadyExists(c.k8sClient.Create(ctx, backup)); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	c.log.Info("Backup resource created or already exists, waiting for completion", "namespace", namespace, "name", backupName)

	// Wait for the backup to complete using watch
	backupToWatch := &unstructured.Unstructured{}
	backupToWatch.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "postgresql.cnpg.io",
		Version: "v1",
		Kind:    "Backup",
	})
	backupToWatch.SetName(backupName)
	backupToWatch.SetNamespace(namespace)

	backupList := &unstructured.UnstructuredList{}
	backupList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "postgresql.cnpg.io",
		Version: "v1",
		Kind:    "BackupList",
	})

	return WatchUntilDone(ctx, c.k8sClient, backupToWatch, backupList, c.timeout, c.checkDone, c.checkSuccess, c.log)
}

// checkDone returns true if the CNPG backup has reached a terminal state
func (c *CNPGBackupRunner) checkDone(obj client.Object) bool {
	backup, ok := TypeAssertObject[*unstructured.Unstructured](obj, "unstructured.Unstructured", c.log)
	if !ok {
		return false
	}

	phase, found, err := unstructured.NestedString(backup.Object, "status", "phase")
	if err != nil {
		c.log.Error(err, "Failed to extract phase from backup status")
		return false
	}

	if !found {
		return false
	}

	// CNPG backup phases: pending, running, completed, failed
	return phase == "completed" || phase == "failed"
}

// checkSuccess verifies the CNPG backup completed successfully
func (c *CNPGBackupRunner) checkSuccess(obj client.Object) error {
	backup, ok := TypeAssertObject[*unstructured.Unstructured](obj, "unstructured.Unstructured", c.log)
	if !ok {
		return fmt.Errorf("unexpected object type: expected unstructured.Unstructured, got %T", obj)
	}

	phase, found, err := unstructured.NestedString(backup.Object, "status", "phase")
	if err != nil {
		return fmt.Errorf("failed to extract phase from backup status: %w", err)
	}

	if !found {
		return fmt.Errorf("backup status phase not found")
	}

	if phase == "completed" {
		c.log.Info("Backup completed successfully",
			"namespace", backup.GetNamespace(),
			"name", backup.GetName())
		return nil
	}

	if phase == "failed" {
		// Try to get error information from status
		errorMsg, _, _ := unstructured.NestedString(backup.Object, "status", "error")
		if errorMsg != "" {
			return fmt.Errorf("backup failed: %s", errorMsg)
		}
		return fmt.Errorf("backup failed with no detailed error information")
	}

	// Unknown phase
	return fmt.Errorf("backup finished with unknown phase: %s", phase)
}
