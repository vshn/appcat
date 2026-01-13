package backup

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// StackGresBackupRunner handles creating and waiting for StackGres backup jobs
type StackGresBackupRunner struct {
	BaseRunner
}

// NewStackGresBackupRunner creates a new StackGresBackupRunner
func NewStackGresBackupRunner(c client.WithWatch, log logr.Logger) *StackGresBackupRunner {
	return &StackGresBackupRunner{
		BaseRunner: NewBaseRunner(c, log),
	}
}

// RunBackup finds the StackGres backup CronJob in the namespace and creates a one-time Job from it
func (s *StackGresBackupRunner) RunBackup(ctx context.Context, namespace, jobName string) error {
	// Check if the SGCluster is suspended (instances == 0)
	clusterList := &stackgresv1.SGClusterList{}
	if err := s.k8sClient.List(ctx, clusterList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("failed to list SGClusters: %w", err)
	}

	if len(clusterList.Items) == 0 {
		s.log.Info("No SGCluster found in namespace, skipping backup", "namespace", namespace)
		return nil
	}

	cluster := &clusterList.Items[0]
	if IsClusterSuspended(cluster.Spec.Instances, s.log, namespace) {
		return nil
	}

	s.log.Info("Finding StackGres backup CronJob", "namespace", namespace)

	// List all CronJobs in the namespace with the backup label selector
	cronJobList := &batchv1.CronJobList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			"stackgres.io/scheduled-backup": "true",
		},
	}

	if err := s.k8sClient.List(ctx, cronJobList, listOpts...); err != nil {
		return fmt.Errorf("failed to list StackGres backup CronJobs: %w", err)
	}

	if len(cronJobList.Items) == 0 {
		s.log.Info("No StackGres backup CronJob found, skipping backup", "namespace", namespace)
		return nil
	}

	// Use the first cronjob found (there should typically only be one per instance)
	cronJob := &cronJobList.Items[0]
	s.log.Info("Found StackGres backup CronJob", "name", cronJob.Name, "namespace", namespace)

	// Create a one-off Job based on the CronJob's job template
	s.log.Info("Creating pre-maintenance backup job", "namespace", namespace, "name", jobName)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"appcat.vshn.io/backup-type": "pre-maintenance",
				"app":                        "StackGresBackup",
			},
			Annotations: cronJob.Spec.JobTemplate.Annotations,
		},
		Spec: cronJob.Spec.JobTemplate.Spec,
	}

	// Create the job resource
	if err := client.IgnoreAlreadyExists(s.k8sClient.Create(ctx, job)); err != nil {
		return fmt.Errorf("failed to create backup job: %w", err)
	}

	s.log.Info("Backup job created or already exists, waiting for completion", "namespace", namespace, "name", jobName)

	// Wait for the job to complete using watch
	jobToWatch := &batchv1.Job{}
	jobToWatch.SetName(jobName)
	jobToWatch.SetNamespace(namespace)

	return WatchUntilDone(ctx, s.k8sClient, jobToWatch, &batchv1.JobList{}, s.timeout, s.checkDone, s.checkSuccess, s.log)
}

// checkDone returns true if the job has reached a terminal state
func (s *StackGresBackupRunner) checkDone(obj client.Object) bool {
	job, ok := TypeAssertObject[*batchv1.Job](obj, "batchv1.Job", s.log)
	if !ok {
		return false
	}

	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete || condition.Type == batchv1.JobFailed {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// checkSuccess verifies the job completed successfully
func (s *StackGresBackupRunner) checkSuccess(obj client.Object) error {
	job, ok := TypeAssertObject[*batchv1.Job](obj, "batchv1.Job", s.log)
	if !ok {
		return fmt.Errorf("unexpected object type: expected batchv1.Job, got %T", obj)
	}

	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			s.log.Info("Backup job completed successfully",
				"namespace", job.Namespace,
				"name", job.Name)
			return nil
		}

		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return fmt.Errorf("backup job failed: %s (reason: %s)", condition.Message, condition.Reason)
		}
	}

	// Job is marked complete but no clear success/failure condition found
	if job.Status.Failed > 0 {
		return fmt.Errorf("backup job failed with %d failed pods", job.Status.Failed)
	}

	if job.Status.Succeeded > 0 {
		s.log.Info("Backup job completed successfully",
			"namespace", job.Namespace,
			"name", job.Name)
		return nil
	}

	// Shouldn't happen, but handle just in case
	return fmt.Errorf("backup job finished with unknown status: %+v", job.Status)
}
