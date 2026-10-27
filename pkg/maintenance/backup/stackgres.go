package backup

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewStackGresWaiter creates a Waiter for the Jobs of the StackGres scheduled backup
func NewStackGresWaiter(c client.Client, log logr.Logger) *Waiter {
	return NewWaiter(c, StackGresBackupsInProgress, &batchv1.Job{}, log)
}

// StackGresBackupsInProgress returns the Jobs of the StackGres backup CronJob in the namespace that haven't finished yet
func StackGresBackupsInProgress(ctx context.Context, c client.Client, namespace string, log logr.Logger) ([]string, error) {
	clusterList := &stackgresv1.SGClusterList{}
	if err := c.List(ctx, clusterList, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list SGClusters: %w", err)
	}
	if len(clusterList.Items) == 0 {
		log.Info("No SGCluster found in namespace, no backups to wait for", "namespace", namespace)
		return nil, nil
	}
	if IsClusterSuspended(clusterList.Items[0].Spec.Instances, log, namespace) {
		return nil, nil
	}

	cronJobList := &batchv1.CronJobList{}
	if err := c.List(ctx, cronJobList, client.InNamespace(namespace), client.MatchingLabels{
		"stackgres.io/scheduled-backup": "true",
	}); err != nil {
		return nil, fmt.Errorf("failed to list StackGres backup CronJobs: %w", err)
	}
	if len(cronJobList.Items) == 0 {
		return nil, nil
	}

	cronJobs := map[types.UID]bool{}
	for _, cj := range cronJobList.Items {
		cronJobs[cj.UID] = true
	}

	jobList := &batchv1.JobList{}
	if err := c.List(ctx, jobList, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list Jobs: %w", err)
	}

	running := []string{}
	for _, job := range jobList.Items {
		if !job.DeletionTimestamp.IsZero() || !ownedByAny(job.OwnerReferences, cronJobs) || isJobFinished(&job) {
			continue
		}
		running = append(running, job.Name)
	}
	return running, nil
}

// ownedByAny returns true if one of the owner references points to one of the owners
func ownedByAny(refs []metav1.OwnerReference, owners map[types.UID]bool) bool {
	for _, ref := range refs {
		if owners[ref.UID] {
			return true
		}
	}
	return false
}

// isJobFinished returns true if the job has reached a terminal state
func isJobFinished(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if (condition.Type == batchv1.JobComplete || condition.Type == batchv1.JobFailed) && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
