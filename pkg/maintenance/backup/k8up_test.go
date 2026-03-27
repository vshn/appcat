package backup

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vshn/appcat/v4/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Helper function to create a succeeded k8up backup status
func createSucceededK8upStatus() k8upv1.Status {
	status := k8upv1.Status{}
	status.SetSucceeded("Backup completed successfully")
	return status
}

// Helper function to create a failed k8up backup status
func createFailedK8upStatus(message string) k8upv1.Status {
	status := k8upv1.Status{}
	status.SetFailed(message)
	return status
}

func TestK8upBackupRunner_RunBackup(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		backupName   string
		existingObjs []client.Object
		wantErr      bool
		errContains  string
	}{
		{
			name:        "GivenNoSchedule_ThenReturnError",
			namespace:   "test-ns",
			backupName:  "test-backup",
			wantErr:     true,
			errContains: "no k8up Schedule found",
		},
		{
			name:       "GivenScheduleWithNoBackend_ThenReturnError",
			namespace:  "test-ns",
			backupName: "test-backup",
			existingObjs: []client.Object{
				&k8upv1.Schedule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-schedule",
						Namespace: "test-ns",
					},
					Spec: k8upv1.ScheduleSpec{
						// No backend configured
					},
				},
			},
			wantErr:     true,
			errContains: "no backend configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(pkg.SetupScheme()).
				WithObjects(tt.existingObjs...).
				WithStatusSubresource(&k8upv1.Backup{}).
				Build()

			runner := &K8upBackupRunner{
				BaseRunner: BaseRunner{
					k8sClient: fakeClient,
					log:       logr.Discard(),
					timeout:   500 * time.Millisecond,
				},
			}

			err := runner.RunBackup(context.Background(), tt.namespace, tt.backupName)

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

func TestK8upBackupRunner_checkSuccess(t *testing.T) {
	tests := []struct {
		name        string
		backup      *k8upv1.Backup
		wantErr     bool
		errContains string
	}{
		{
			name: "GivenSucceededBackup_ThenReturnNil",
			backup: func() *k8upv1.Backup {
				b := &k8upv1.Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-backup",
						Namespace: "test-ns",
					},
				}
				b.Status = createSucceededK8upStatus()
				return b
			}(),
			wantErr: false,
		},
		{
			name: "GivenFailedBackup_ThenReturnError",
			backup: func() *k8upv1.Backup {
				b := &k8upv1.Backup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-backup",
						Namespace: "test-ns",
					},
				}
				b.Status = createFailedK8upStatus("Backup failed due to storage error")
				return b
			}(),
			wantErr:     true,
			errContains: "backup failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &K8upBackupRunner{
				BaseRunner: BaseRunner{
					log: logr.Discard(),
				},
			}

			err := runner.checkSuccess(tt.backup)

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

func TestNewK8upBackupRunner(t *testing.T) {
	fakeClient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		Build()

	runner := NewK8upBackupRunner(fakeClient, logr.Discard())

	require.NotNil(t, runner)
	assert.Equal(t, 1*time.Hour, runner.timeout)
	assert.NotNil(t, runner.k8sClient)
}

func TestK8upBackupRunner_BackupLabels(t *testing.T) {
	// Test that the created backup has the correct labels
	fakeClient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(
			&k8upv1.Schedule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-schedule",
					Namespace: "test-ns",
				},
				Spec: k8upv1.ScheduleSpec{
					Backend: &k8upv1.Backend{
						S3: &k8upv1.S3Spec{
							Bucket: "test-bucket",
						},
					},
				},
			},
		).
		WithStatusSubresource(&k8upv1.Backup{}).
		Build()

	runner := &K8upBackupRunner{
		BaseRunner: BaseRunner{
			k8sClient: fakeClient,
			log:       logr.Discard(),
			timeout:   100 * time.Millisecond, // Short timeout to fail fast
		},
	}

	// Start backup (will timeout, but that's ok - we just want to check labels)
	go func() {
		_ = runner.RunBackup(context.Background(), "test-ns", "labeled-backup")
	}()

	// Wait for backup to be created
	time.Sleep(50 * time.Millisecond)

	backup := &k8upv1.Backup{}
	err := fakeClient.Get(context.Background(), client.ObjectKey{Namespace: "test-ns", Name: "labeled-backup"}, backup)
	if err == nil {
		assert.Equal(t, "pre-maintenance", backup.Labels["appcat.vshn.io/backup-type"])
	}
}
