package backup

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	cnpgv1 "github.com/vshn/appcat/v4/apis/cnpg/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cnpgBackupPhaseWalArchivingFailing is a terminal CNPG phase that isn't part of our API copy
const cnpgBackupPhaseWalArchivingFailing cnpgv1.BackupPhase = "walArchivingFailing"

// NewCNPGWaiter creates a Waiter for CNPG Backup resources
func NewCNPGWaiter(c client.Client, log logr.Logger) *Waiter {
	return NewWaiter(c, CNPGBackupsInProgress, &cnpgv1.Backup{}, log)
}

// CNPGBackupsInProgress returns the CNPG Backups in the namespace that haven't reached a terminal phase
func CNPGBackupsInProgress(ctx context.Context, c client.Client, namespace string, log logr.Logger) ([]string, error) {
	clusterList := &cnpgv1.ClusterList{}
	if err := c.List(ctx, clusterList, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list CNPG Clusters: %w", err)
	}
	if len(clusterList.Items) == 0 {
		log.Info("No CNPG Cluster found in namespace, no backups to wait for", "namespace", namespace)
		return nil, nil
	}
	// Pending backups never start on a suspended cluster
	if IsClusterSuspended(clusterList.Items[0].Spec.Instances, log, namespace) {
		return nil, nil
	}

	backups := &cnpgv1.BackupList{}
	if err := c.List(ctx, backups, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list CNPG Backups: %w", err)
	}

	running := []string{}
	for _, b := range backups.Items {
		if !b.DeletionTimestamp.IsZero() {
			continue
		}
		switch b.Status.Phase {
		case cnpgv1.BackupPhaseCompleted, cnpgv1.BackupPhaseFailed, cnpgBackupPhaseWalArchivingFailing:
			continue
		}
		running = append(running, b.Name)
	}
	return running, nil
}
