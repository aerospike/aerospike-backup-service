package service

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
)

// configRetriever is used to read Aerospike configuration from backup.
type configRetriever struct {
	backends BackendsHolder
}

// RetrieveConfiguration return backed up Aerospike configuration.
func (cr *configRetriever) RetrieveConfiguration(ctx context.Context, routine string, toTime *time.Time,
) ([]byte, error) {
	backend, found := cr.backends.GetReader(routine)
	if !found {
		return nil, fmt.Errorf("%w: routine %s", errBackendNotFound, routine)
	}

	lastFullBackup, err := backend.FindLastFullBackup(ctx, toTime)
	if err != nil {
		return nil, fmt.Errorf("failed retrieve configuration: %w", err)
	}

	configPath, err := calculateConfigurationBackupPath(lastFullBackup.Key)
	if err != nil {
		return nil, err
	}

	return backend.ReadClusterConfiguration(ctx, configPath)
}

func calculateConfigurationBackupPath(backupKey string) (string, error) {
	_, path, err := util.ParseS3Path(backupKey)
	if err != nil {
		return "", err
	}
	// Move up two directories
	base := filepath.Dir(filepath.Dir(path))
	// Join new directory 'config' with the new base
	return filepath.Join(base, configurationBackupDirectory), nil
}
