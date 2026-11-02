package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// BackupTypeLabelKey marks the purpose of a backup resource created by AppCat
	BackupTypeLabelKey = "appcat.vshn.io/backup-type"
	// BackupTypePreMaintenance is the label value of the former pre-maintenance backups
	BackupTypePreMaintenance = "pre-maintenance"
	// MaxBackupAge is how long a backup may run before it's considered stuck.
	// Stuck backups don't block the maintenance, otherwise it would never run.
	MaxBackupAge = 24 * time.Hour
)

// InProgressFunc returns the names of the backups in the namespace that are still running
type InProgressFunc func(ctx context.Context, c client.Client, namespace string, log logr.Logger) ([]string, error)

// Waiter blocks the maintenance until no backup is running in the instance namespace anymore.
// A maintenance can restart the instance, which would break a running backup.
type Waiter struct {
	k8sClient  client.Client
	log        logr.Logger
	timeout    time.Duration
	interval   time.Duration
	inProgress InProgressFunc
	// leftoverType is the resource type the former pre-maintenance backups were created as
	leftoverType client.Object
}

// NewWaiter creates a new Waiter with default settings
func NewWaiter(c client.Client, inProgress InProgressFunc, leftoverType client.Object, log logr.Logger) *Waiter {
	return &Waiter{
		k8sClient:    c,
		log:          log,
		timeout:      6 * time.Hour,
		interval:     30 * time.Second,
		inProgress:   inProgress,
		leftoverType: leftoverType,
	}
}

// WaitForRunningBackups waits until no backup is running in the instance namespace.
// It fails if there are still backups running after the timeout.
func (w *Waiter) WaitForRunningBackups(ctx context.Context) error {
	namespace := viper.GetString("INSTANCE_NAMESPACE")
	if namespace == "" {
		return fmt.Errorf("INSTANCE_NAMESPACE not set")
	}

	// Stuck leftovers would block every maintenance until the timeout
	if err := CleanupPreMaintenanceBackups(ctx, w.k8sClient, w.leftoverType, namespace, w.log); err != nil {
		w.log.Error(err, "Could not remove leftover pre-maintenance backups", "namespace", namespace)
	}

	var running []string
	var lastErr error
	err := wait.PollUntilContextTimeout(ctx, w.interval, w.timeout, true, func(ctx context.Context) (bool, error) {
		running, lastErr = w.inProgress(ctx, w.k8sClient, namespace, w.log)
		// Transient API errors shouldn't abort the maintenance, retry until the timeout
		if lastErr != nil {
			w.log.Error(lastErr, "Could not check for running backups, retrying", "namespace", namespace)
			return false, nil
		}
		if len(running) == 0 {
			return true, nil
		}
		w.log.Info("Backups in progress, waiting for them to finish before maintenance", "namespace", namespace, "backups", running)
		return false, nil
	})
	if wait.Interrupted(err) && lastErr != nil {
		return fmt.Errorf("failed to check for running backups: %w", lastErr)
	}
	if wait.Interrupted(err) {
		return fmt.Errorf("backups still in progress after %s: %v", w.timeout, running)
	}
	if err != nil {
		return fmt.Errorf("failed to check for running backups: %w", err)
	}

	w.log.Info("No backups in progress", "namespace", namespace)
	return nil
}

// CleanupPreMaintenanceBackups removes the backup resources the former pre-maintenance backup left behind.
// TODO: remove once all instances went through a maintenance with this version.
func CleanupPreMaintenanceBackups(ctx context.Context, c client.Client, obj client.Object, namespace string, log logr.Logger) error {
	log.Info("Removing leftover pre-maintenance backups", "namespace", namespace)

	err := c.DeleteAllOf(
		ctx, obj,
		client.InNamespace(namespace),
		client.MatchingLabels{BackupTypeLabelKey: BackupTypePreMaintenance},
		client.PropagationPolicy(metav1.DeletePropagationBackground),
	)
	if err != nil {
		return fmt.Errorf("failed to remove leftover pre-maintenance backups: %w", err)
	}

	return nil
}

// IsBackupStuck returns true if the backup was created more than MaxBackupAge ago
func IsBackupStuck(obj metav1.Object, log logr.Logger) bool {
	created := obj.GetCreationTimestamp()
	if created.IsZero() {
		return false
	}
	age := time.Since(created.Time)
	if age < MaxBackupAge {
		return false
	}
	log.Info("Backup is running for too long, considering it stuck and not waiting for it",
		"namespace", obj.GetNamespace(), "name", obj.GetName(), "age", age.Round(time.Second).String())
	return true
}

// IsClusterSuspended checks if a cluster is suspended (instances == 0) and logs appropriately
func IsClusterSuspended(instances int, log logr.Logger, namespace string) bool {
	if instances == 0 {
		log.Info("Cluster is suspended (instances=0), backups can't run", "namespace", namespace)
		return true
	}
	return false
}
