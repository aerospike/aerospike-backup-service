package service

import (
	"sync/atomic"

	"github.com/aerospike/backup/pkg/model"
)

// BackupBackend allows access to back up storage
type BackupBackend interface {
	BackupListReader

	// readState reads and returns the state for the backup.
	readState() *model.BackupState

	// writeState writes the state object for the backup.
	writeState(*model.BackupState) error

	// writeBackupCreationTime writes creation time in the metadata file under the backup folder.
	writeBackupMetadata(path string, metadata model.BackupMetadata) error

	// CreateFolder creates folder with given path.
	CreateFolder(path string)

	// DeleteFolder removes file with a given basePath.
	DeleteFolder(path string) error

	// FullBackupInProgress indicates whether a full backup is in progress.
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
