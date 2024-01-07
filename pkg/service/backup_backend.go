package service

import (
	"github.com/aerospike/backup/pkg/model"
	"sync/atomic"
)

// BackupBackend represents a backup backend handler.
type BackupBackend interface {
	BackupListReader

	// readState reads and returns the state for the backup.
	readState() *model.BackupState

	// writeState writes the state object for the backup.
	writeState(*model.BackupState) error

	// CleanDir cleans the directory with the given name.
	CleanDir(name string) error

	// DeleteFile removes file with a given path.
	DeleteFile(path string) error

	FullBackupInProgress() *atomic.Bool
}

// BackupListReader allows to read list of existing backups
type BackupListReader interface {
	// FullBackupList returns a list of available full backups.
	// The parameters are timestamp filters by creation time, where from is inclusive
	// and to is exclusive.
	FullBackupList(from, to int64) ([]model.BackupDetails, error)

	// IncrementalBackupList returns a list of available incremental backups.
	IncrementalBackupList() ([]model.BackupDetails, error)
}

func BuildBackupBackends(config *model.Config) map[string]BackupBackend {
	backends := map[string]BackupBackend{}
	for routineName := range config.BackupRoutines {
		backends[routineName] = newBackend(config, routineName)
	}
	return backends
}

func newBackend(config *model.Config, routineName string) BackupBackend {
	backupRoutine := config.BackupRoutines[routineName]
	storage := config.Storage[backupRoutine.Storage]
	backupPolicy := config.BackupPolicies[backupRoutine.BackupPolicy]
	var backend BackupBackend
	switch storage.Type {
	case model.Local:
		backend = NewBackupBackendLocal(storage, backupPolicy)
	case model.S3:
		backend = NewBackupBackendS3(storage, backupPolicy)
	}
	return backend
}
