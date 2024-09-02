package service

import (
	"fmt"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"path/filepath"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
)

type configRetriever struct {
	backends BackendsHolder
}

// RetrieveConfiguration return backed up Aerospike configuration.
func (cr *configRetriever) RetrieveConfiguration(routine string, toTime time.Time,
) ([]byte, error) {
	backend, found := cr.backends.GetReader(routine)
	if !found {
		return nil, fmt.Errorf("%w: routine %s", errBackendNotFound, routine)
	}

	fullBackups, err := backend.FindLastFullBackup(toTime)
	if err != nil {
		return nil, fmt.Errorf("failed retrieve configuration: %w", err)
	}

	// fullBackups has backups for multiple namespaces, but same timestamp,
	// they share the same configuration.
	lastFullBackup := fullBackups[0]
	configPath, err := calculateConfigurationBackupPath(*lastFullBackup.Key)
	if err != nil {
		return nil, err
	}

	return backend.ReadClusterConfiguration(configPath)
}

func calculateConfigurationBackupPath(backupKey string) (string, error) {
	_, path, err := util.ParseS3Path(backupKey)
	if err != nil {
		return "", err
	}
	// Move up two directories
	base := filepath.Dir(filepath.Dir(path))
	// Join new directory 'config' with the new base
	return filepath.Join(base, model.ConfigurationBackupDirectory), nil
}
