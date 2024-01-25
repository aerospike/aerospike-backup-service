package service

import (
	"sync/atomic"

	"github.com/aerospike/backup/pkg/model"
)

// BackupBackend allows access to back up storage
type BackupBackend interface {
	BackupListReader
	StorageAccessor

	// readState reads and returns the state for the backup.
	readState() *model.BackupState

	// writeState writes the state object for the backup.
	writeState(*model.BackupState) error

	// writeBackupCreationTime writes creation time in the metadata file under the backup folder.
	writeBackupMetadata(path string, metadata model.BackupMetadata) error

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

type StorageAccessor interface {
	readBackupState(path string, state *model.BackupState) error
	readBackupDetails(path string) (model.BackupDetails, error)
	writeYaml(filePath string, v any) error
	lsDir(path string) ([]string, error)
	// DeleteFolder removes file with a given basePath.
	DeleteFolder(path string) error
	// CreateFolder creates folder with given path.
	CreateFolder(path string)
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
	removeFiles := backupPolicy.RemoveFiles != nil && *backupPolicy.RemoveFiles
	switch storage.Type {
	case model.Local:
		path := *storage.Path
		diskAccessor := NewOS(path)

		return &BackupBackendImpl{
			StorageAccessor:      &diskAccessor,
			path:                 path,
			stateFilePath:        path + "/" + model.StateFileName,
			removeFiles:          removeFiles,
			fullBackupInProgress: &atomic.Bool{},
		}
	case model.S3:
		s3Context, err := NewS3Context(storage)
		if err != nil {
			panic(err)
		}

		return &BackupBackendImpl{
			StorageAccessor:      s3Context,
			path:                 s3Context.path,
			stateFilePath:        s3Context.path + "/" + model.StateFileName,
			removeFiles:          removeFiles,
			fullBackupInProgress: &atomic.Bool{},
		}
	}
	return nil
}
