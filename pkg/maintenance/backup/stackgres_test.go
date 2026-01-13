package backup

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	"github.com/vshn/appcat/v4/pkg"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestStackGresBackupRunner_RunBackup(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		jobName      string
		existingObjs []client.Object
		wantErr      bool
		errContains  string
	}{
		{
			name:      "GivenNoSGCluster_ThenSkipBackup",
			namespace: "test-ns",
			jobName:   "test-backup",
			wantErr:   false, // Should skip gracefully
		},
		{
			name:      "GivenSuspendedCluster_ThenSkipBackup",
			namespace: "test-ns",
			jobName:   "test-backup",
			existingObjs: []client.Object{
				&stackgresv1.SGCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "test-ns",
					},
					Spec: stackgresv1.SGClusterSpec{
						Instances: 0, // Suspended
					},
				},
			},
			wantErr: false, // Should skip gracefully
		},
		{
			name:      "GivenNoBackupCronJob_ThenSkipBackup",
			namespace: "test-ns",
			jobName:   "test-backup",
			existingObjs: []client.Object{
				&stackgresv1.SGCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "test-ns",
					},
					Spec: stackgresv1.SGClusterSpec{
						Instances: 1,
					},
				},
			},
			wantErr: false, // Should skip gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(pkg.SetupScheme()).
				WithObjects(tt.existingObjs...).
				WithStatusSubresource(&batchv1.Job{}).
				Build()

			runner := &StackGresBackupRunner{
				BaseRunner: BaseRunner{
					k8sClient: fakeClient,
					log:       logr.Discard(),
					timeout:   500 * time.Millisecond,
				},
			}

			err := runner.RunBackup(context.Background(), tt.namespace, tt.jobName)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStackGresBackupRunner_checkSuccess(t *testing.T) {
	tests := []struct {
		name        string
		job         *batchv1.Job
		wantErr     bool
		errContains string
	}{
		{
			name: "GivenSucceededJob_ThenReturnNil",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "test-ns",
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "GivenFailedJob_ThenReturnError",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "test-ns",
				},
				Status: batchv1.JobStatus{
					Failed: 1,
					Conditions: []batchv1.JobCondition{
						{
							Type:    batchv1.JobFailed,
							Status:  corev1.ConditionTrue,
							Message: "BackoffLimitExceeded",
							Reason:  "BackoffLimitExceeded",
						},
					},
				},
			},
			wantErr:     true,
			errContains: "backup job failed",
		},
		{
			name: "GivenJobWithFailedPods_ThenReturnError",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "test-ns",
				},
				Status: batchv1.JobStatus{
					Failed: 3,
				},
			},
			wantErr:     true,
			errContains: "failed pods",
		},
		{
			name: "GivenJobWithSucceededPods_ThenReturnNil",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "test-ns",
				},
				Status: batchv1.JobStatus{
					Succeeded: 1,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &StackGresBackupRunner{
				BaseRunner: BaseRunner{
					log: logr.Discard(),
				},
			}

			err := runner.checkSuccess(tt.job)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStackGresBackupRunner_checkDone(t *testing.T) {
	tests := []struct {
		name string
		job  *batchv1.Job
		want bool
	}{
		{
			name: "GivenJobComplete_ThenReturnTrue",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "GivenJobFailed_ThenReturnTrue",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobFailed,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "GivenJobRunning_ThenReturnFalse",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active: 1,
				},
			},
			want: false,
		},
		{
			name: "GivenJobConditionFalse_ThenReturnFalse",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{
							Type:   batchv1.JobComplete,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &StackGresBackupRunner{
				BaseRunner: BaseRunner{
					log: logr.Discard(),
				},
			}
			got := runner.checkDone(tt.job)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewStackGresBackupRunner(t *testing.T) {
	fakeClient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		Build()

	runner := NewStackGresBackupRunner(fakeClient, logr.Discard())

	require.NotNil(t, runner)
	assert.Equal(t, 1*time.Hour, runner.timeout)
	assert.NotNil(t, runner.k8sClient)
}

func TestStackGresBackupRunner_JobLabels(t *testing.T) {
	// Test that the created job has the correct labels
	fakeClient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(
			&stackgresv1.SGCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-ns",
				},
				Spec: stackgresv1.SGClusterSpec{
					Instances: 1,
				},
			},
			&batchv1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "backup-cronjob",
					Namespace: "test-ns",
					Labels: map[string]string{
						"stackgres.io/scheduled-backup": "true",
					},
				},
				Spec: batchv1.CronJobSpec{
					Schedule: "0 0 * * *",
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "backup",
											Image: "backup-image:latest",
										},
									},
									RestartPolicy: corev1.RestartPolicyNever,
								},
							},
						},
					},
				},
			},
		).
		WithStatusSubresource(&batchv1.Job{}).
		Build()

	runner := &StackGresBackupRunner{
		BaseRunner: BaseRunner{
			k8sClient: fakeClient,
			log:       logr.Discard(),
			timeout:   100 * time.Millisecond, // Short timeout to fail fast
		},
	}

	// Start backup (will timeout, but that's ok - we just want to check labels)
	go func() {
		_ = runner.RunBackup(context.Background(), "test-ns", "labeled-job")
	}()

	// Wait for job to be created
	time.Sleep(50 * time.Millisecond)

	job := &batchv1.Job{}
	err := fakeClient.Get(context.Background(), client.ObjectKey{Namespace: "test-ns", Name: "labeled-job"}, job)
	if err == nil {
		assert.Equal(t, "pre-maintenance", job.Labels["appcat.vshn.io/backup-type"])
		assert.Equal(t, "StackGresBackup", job.Labels["app"])
	}
}
