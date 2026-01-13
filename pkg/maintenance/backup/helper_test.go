package backup

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// MockRunner implements the Runner interface for testing
type MockRunner struct {
	runBackupFunc func(ctx context.Context, namespace, backupName string) error
}

func (m *MockRunner) RunBackup(ctx context.Context, namespace, backupName string) error {
	if m.runBackupFunc != nil {
		return m.runBackupFunc(ctx, namespace, backupName)
	}
	return nil
}

func TestHelper_RunBackup(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func()
		runnerFunc  func(ctx context.Context, namespace, backupName string) error
		wantErr     bool
		errContains string
	}{
		{
			name: "GivenSuccessfulBackup_ThenReturnNil",
			setupEnv: func() {
				viper.Set("INSTANCE_NAMESPACE", "test-namespace")
				viper.Set("COMPOSITE_NAME", "test-composite")
			},
			runnerFunc: func(ctx context.Context, namespace, backupName string) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "GivenMissingNamespace_ThenReturnError",
			setupEnv: func() {
				viper.Set("COMPOSITE_NAME", "test-composite")
				// INSTANCE_NAMESPACE not set
			},
			wantErr:     true,
			errContains: "INSTANCE_NAMESPACE not set",
		},
		{
			name: "GivenMissingCompositeName_ThenReturnError",
			setupEnv: func() {
				viper.Set("INSTANCE_NAMESPACE", "test-namespace")
				// COMPOSITE_NAME not set
			},
			wantErr:     true,
			errContains: "COMPOSITE_NAME not set",
		},
		{
			name: "GivenRunnerError_ThenReturnError",
			setupEnv: func() {
				viper.Set("INSTANCE_NAMESPACE", "test-namespace")
				viper.Set("COMPOSITE_NAME", "test-composite")
			},
			runnerFunc: func(ctx context.Context, namespace, backupName string) error {
				return errors.New("runner failed")
			},
			wantErr:     true,
			errContains: "runner failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper before each test
			viper.Reset()

			// Setup test environment
			if tt.setupEnv != nil {
				tt.setupEnv()
			}

			runner := &MockRunner{runBackupFunc: tt.runnerFunc}
			helper := NewHelper(runner, logr.Discard())

			err := helper.RunBackup(context.Background())

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

func TestGetBackupMetadata(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func()
		wantErr     bool
		errContains string
		checkMeta   func(t *testing.T, meta *Metadata)
	}{
		{
			name: "GivenValidEnv_ThenReturnMetadata",
			setupEnv: func() {
				viper.Set("INSTANCE_NAMESPACE", "test-ns")
				viper.Set("COMPOSITE_NAME", "test-comp")
			},
			wantErr: false,
			checkMeta: func(t *testing.T, meta *Metadata) {
				assert.Equal(t, "test-ns", meta.Namespace)
				assert.Equal(t, "test-comp", meta.CompositeName)
				assert.NotEmpty(t, meta.Timestamp)
			},
		},
		{
			name: "GivenMissingNamespace_ThenReturnError",
			setupEnv: func() {
				viper.Set("COMPOSITE_NAME", "test-comp")
			},
			wantErr:     true,
			errContains: "INSTANCE_NAMESPACE not set",
		},
		{
			name: "GivenMissingCompositeName_ThenReturnError",
			setupEnv: func() {
				viper.Set("INSTANCE_NAMESPACE", "test-ns")
			},
			wantErr:     true,
			errContains: "COMPOSITE_NAME not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper
			viper.Reset()

			if tt.setupEnv != nil {
				tt.setupEnv()
			}

			meta, err := GetBackupMetadata()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				if tt.checkMeta != nil {
					tt.checkMeta(t, meta)
				}
			}
		})
	}
}

func TestMetadata_FormatBackupName(t *testing.T) {
	meta := &Metadata{
		Namespace:     "test-ns",
		CompositeName: "my-composite",
		Timestamp:     "20240115-120000",
	}

	name := meta.FormatBackupName()

	assert.Equal(t, "my-composite-premaint-20240115-120000", name)
}
