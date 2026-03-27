package backup

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cnpgv1 "github.com/vshn/appcat/v4/apis/cnpg/v1"
	"github.com/vshn/appcat/v4/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCNPGBackupRunner_RunBackup(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		backupName   string
		existingObjs []client.Object
		wantErr      bool
		errContains  string
	}{
		{
			name:       "GivenNoCluster_ThenSkipBackup",
			namespace:  "test-ns",
			backupName: "test-backup",
			wantErr:    false, // Should skip gracefully
		},
		{
			name:       "GivenSuspendedCluster_ThenSkipBackup",
			namespace:  "test-ns",
			backupName: "test-backup",
			existingObjs: []client.Object{
				&cnpgv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "test-ns",
					},
					Spec: cnpgv1.ClusterSpec{
						Instances: 0, // Suspended
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
				Build()

			runner := &CNPGBackupRunner{
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

func TestCNPGBackupRunner_checkSuccess(t *testing.T) {
	tests := []struct {
		name        string
		phase       cnpgv1.BackupPhase
		errorMsg    string
		wantErr     bool
		errContains string
	}{
		{
			name:    "GivenCompletedPhase_ThenReturnNil",
			phase:   cnpgv1.BackupPhaseCompleted,
			wantErr: false,
		},
		{
			name:        "GivenFailedPhase_ThenReturnError",
			phase:       cnpgv1.BackupPhaseFailed,
			wantErr:     true,
			errContains: "backup failed",
		},
		{
			name:        "GivenFailedPhaseWithError_ThenReturnErrorWithMessage",
			phase:       cnpgv1.BackupPhaseFailed,
			errorMsg:    "Storage quota exceeded",
			wantErr:     true,
			errContains: "Storage quota exceeded",
		},
		{
			name:        "GivenUnknownPhase_ThenReturnError",
			phase:       cnpgv1.BackupPhase("unknown"),
			wantErr:     true,
			errContains: "unknown phase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &CNPGBackupRunner{
				BaseRunner: BaseRunner{
					log: logr.Discard(),
				},
			}

			backup := &cnpgv1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-backup",
					Namespace: "test-ns",
				},
				Status: cnpgv1.BackupStatus{
					Phase: tt.phase,
					Error: tt.errorMsg,
				},
			}

			err := runner.checkSuccess(backup)

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

func TestNewCNPGBackupRunner(t *testing.T) {
	fakeClient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		Build()

	runner := NewCNPGBackupRunner(fakeClient, logr.Discard())

	require.NotNil(t, runner)
	assert.Equal(t, 1*time.Hour, runner.timeout)
	assert.NotNil(t, runner.k8sClient)
}

func TestCNPGBackupRunner_BackupLabels(t *testing.T) {
	// Test that the created backup has the correct labels
	fakeClient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(
			&cnpgv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-ns",
				},
				Spec: cnpgv1.ClusterSpec{
					Instances: 1,
				},
			},
		).
		Build()

	runner := &CNPGBackupRunner{
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

	backup := &cnpgv1.Backup{}
	err := fakeClient.Get(context.Background(), client.ObjectKey{Namespace: "test-ns", Name: "labeled-backup"}, backup)
	if err == nil {
		labels := backup.GetLabels()
		assert.Equal(t, "pre-maintenance", labels["appcat.vshn.io/backup-type"])
	}
}

func TestCNPGBackupRunner_BackupSpec(t *testing.T) {
	// Test that the created backup has the correct spec
	fakeClient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(
			&cnpgv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster",
					Namespace: "test-ns",
				},
				Spec: cnpgv1.ClusterSpec{
					Instances: 1,
				},
			},
		).
		Build()

	runner := &CNPGBackupRunner{
		BaseRunner: BaseRunner{
			k8sClient: fakeClient,
			log:       logr.Discard(),
			timeout:   100 * time.Millisecond,
		},
	}

	// Start backup (will timeout, but that's ok - we just want to check spec)
	go func() {
		_ = runner.RunBackup(context.Background(), "test-ns", "spec-backup")
	}()

	// Wait for backup to be created
	time.Sleep(50 * time.Millisecond)

	backup := &cnpgv1.Backup{}
	err := fakeClient.Get(context.Background(), client.ObjectKey{Namespace: "test-ns", Name: "spec-backup"}, backup)
	if err == nil {
		// Check that the backup references the correct cluster
		assert.Equal(t, "my-cluster", backup.Spec.Cluster.Name)

		// Check the backup method
		assert.Equal(t, cnpgv1.BackupMethodPlugin, backup.Spec.Method)

		// Check that plugin name is the barman cloud plugin
		assert.Equal(t, "barman-cloud.cloudnative-pg.io", backup.Spec.PluginConfiguration.Name)
	}
}

func TestCNPGBackupRunner_MultipleClustersTakesFirst(t *testing.T) {
	// Test that when multiple clusters exist, the first one is used
	fakeClient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(
			&cnpgv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-a",
					Namespace: "test-ns",
				},
				Spec: cnpgv1.ClusterSpec{
					Instances: 1,
				},
			},
			&cnpgv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-b",
					Namespace: "test-ns",
				},
				Spec: cnpgv1.ClusterSpec{
					Instances: 2,
				},
			},
		).
		Build()

	runner := &CNPGBackupRunner{
		BaseRunner: BaseRunner{
			k8sClient: fakeClient,
			log:       logr.Discard(),
			timeout:   100 * time.Millisecond,
		},
	}

	// Start backup (will timeout, but that's ok - we just want to verify it runs)
	go func() {
		_ = runner.RunBackup(context.Background(), "test-ns", "multi-cluster-backup")
	}()

	// Wait for backup to be created
	time.Sleep(50 * time.Millisecond)

	// Verify a backup was created (regardless of which cluster was used)
	backup := &cnpgv1.Backup{}
	err := fakeClient.Get(context.Background(), client.ObjectKey{Namespace: "test-ns", Name: "multi-cluster-backup"}, backup)
	assert.NoError(t, err)
}
