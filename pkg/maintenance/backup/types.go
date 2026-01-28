package backup

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Runner defines the interface for backup implementations
type Runner interface {
	// RunBackup creates and waits for a backup to complete
	RunBackup(ctx context.Context, namespace, backupName string) error
}

// Metadata contains common information needed for creating backups
type Metadata struct {
	Namespace     string
	CompositeName string
	Timestamp     string
}

// GetBackupMetadata extracts common backup metadata from environment variables
func GetBackupMetadata() (*Metadata, error) {
	namespace := viper.GetString("INSTANCE_NAMESPACE")
	if namespace == "" {
		return nil, fmt.Errorf("INSTANCE_NAMESPACE not set")
	}

	compositeName := viper.GetString("COMPOSITE_NAME")
	if compositeName == "" {
		return nil, fmt.Errorf("COMPOSITE_NAME not set")
	}

	return &Metadata{
		Namespace:     namespace,
		CompositeName: compositeName,
		Timestamp:     time.Now().Format("20060102-150405"),
	}, nil
}

// FormatBackupName creates a consistent backup name across all services
func (m *Metadata) FormatBackupName() string {
	return fmt.Sprintf("premaint-%s", m.Timestamp)
}
