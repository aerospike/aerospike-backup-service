package service

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/util"
)

type configRetriever struct {
	backends BackendsHolder
}

// RetrieveConfiguration return backed up Aerospike configuration.
func (cr *configRetriever) RetrieveConfiguration(routine string, toTime time.Time) ([]byte, error) {
	backend, found := cr.backends.GetReader(routine)
	if !found {
		return nil, fmt.Errorf("backend '%s' not found for configuration retrieval", routine)
	}

	fullBackups, err := backend.FindLastFullBackup(toTime)
	if err != nil || len(fullBackups) == 0 {
		return nil, fmt.Errorf("last full backup not found: %v", err)
	}

	// fullBackups has backups for multiple namespaces, but same timestamp, they share same configuration.
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
