package backup

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// K8upBackupRunner handles creating and waiting for k8up Backup resources
type K8upBackupRunner struct {
	BaseRunner
}

// NewK8upBackupRunner creates a new K8upBackupRunner
func NewK8upBackupRunner(c client.WithWatch, log logr.Logger) *K8upBackupRunner {
	return &K8upBackupRunner{
		BaseRunner: NewBaseRunner(c, log),
	}
}

// RunBackup finds the k8up Schedule in the namespace, extracts its backend config,
// and creates a one-off Backup resource to run before maintenance.
func (k *K8upBackupRunner) RunBackup(ctx context.Context, namespace, backupName string) error {
	k.log.Info("Finding k8up Schedule to extract backup configuration", "namespace", namespace)

	// List all Schedules in the namespace
	scheduleList := &k8upv1.ScheduleList{}
	if err := k.k8sClient.List(ctx, scheduleList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("failed to list k8up Schedules: %w", err)
	}

	if len(scheduleList.Items) == 0 {
		return fmt.Errorf("no k8up Schedule found in namespace %s", namespace)
	}

	// Use the first schedule found (there should typically only be one per instance)
	schedule := &scheduleList.Items[0]
	k.log.Info("Found k8up Schedule", "name", schedule.Name, "namespace", namespace)

	if schedule.Spec.Backend == nil {
		return fmt.Errorf("k8up Schedule %s has no backend configuration", schedule.Name)
	}

	// Create a one-off Backup using the same backend configuration
	k.log.Info("Creating pre-maintenance backup", "namespace", namespace, "name", backupName)

	backup := &k8upv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: namespace,
			Labels: map[string]string{
				"appcat.vshn.io/backup-type": "pre-maintenance",
			},
		},
		Spec: k8upv1.BackupSpec{
			RunnableSpec: k8upv1.RunnableSpec{
				Backend: schedule.Spec.Backend,
			},
			FailedJobsHistoryLimit:     ptr.To(2),
			SuccessfulJobsHistoryLimit: ptr.To(2),
		},
	}

	// Create the backup resource
	if err := client.IgnoreAlreadyExists(k.k8sClient.Create(ctx, backup)); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	k.log.Info("Backup resource created or already exists, waiting for completion", "namespace", namespace, "name", backupName)

	// Wait for the backup to complete using watch
	backupToWatch := &k8upv1.Backup{}
	backupToWatch.SetName(backupName)
	backupToWatch.SetNamespace(namespace)

	return WatchUntilDone(ctx, k.k8sClient, backupToWatch, &k8upv1.BackupList{}, k.timeout, k.checkDone, k.checkSuccess, k.log)
}

// checkDone returns true if the k8up backup has finished (success or failure)
func (k *K8upBackupRunner) checkDone(obj client.Object) bool {
	backup, ok := TypeAssertObject[*k8upv1.Backup](obj, "k8upv1.Backup", k.log)
	if !ok {
		return false
	}
	return backup.Status.HasFinished()
}

// checkSuccess returns nil if the k8up backup succeeded, or an error if it failed
func (k *K8upBackupRunner) checkSuccess(obj client.Object) error {
	backup, ok := TypeAssertObject[*k8upv1.Backup](obj, "k8upv1.Backup", k.log)
	if !ok {
		return fmt.Errorf("unexpected object type: expected k8upv1.Backup, got %T", obj)
	}

	// Use k8up's built-in status helper methods
	if backup.Status.HasSucceeded() {
		k.log.Info("Backup completed successfully",
			"namespace", backup.Namespace,
			"name", backup.Name)
		return nil
	}

	if backup.Status.HasFailed() {
		// Try to find a condition with more details
		for _, condition := range backup.Status.Conditions {
			if condition.Status == metav1.ConditionFalse {
				return fmt.Errorf("backup failed: %s (reason: %s)", condition.Message, condition.Reason)
			}
		}
		return fmt.Errorf("backup failed with no detailed error information")
	}

	// Finished but no clear status - this shouldn't happen
	return fmt.Errorf("backup finished with unknown status: %+v", backup.Status)
}
