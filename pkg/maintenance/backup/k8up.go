package backup

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewK8upWaiter creates a Waiter for k8up Backup resources
func NewK8upWaiter(c client.Client, log logr.Logger) *Waiter {
	return NewWaiter(c, K8upBackupsInProgress, &k8upv1.Backup{}, log)
}

// K8upBackupsInProgress returns the k8up Backups in the namespace that haven't finished yet
func K8upBackupsInProgress(ctx context.Context, c client.Client, namespace string, _ logr.Logger) ([]string, error) {
	backups := &k8upv1.BackupList{}
	if err := c.List(ctx, backups, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list k8up Backups: %w", err)
	}

	running := []string{}
	for _, b := range backups.Items {
		if !b.DeletionTimestamp.IsZero() || b.Status.HasFinished() {
			continue
		}
		running = append(running, b.Name)
	}
	return running, nil
}
