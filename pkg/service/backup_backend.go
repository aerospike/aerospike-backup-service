package service

import (
	"github.com/aerospike/backup/pkg/model"
)

// BackupBackend represents a backup backend handler.
type BackupBackend interface {
	BackupListReader

	// readState reads and returns the state for the backup.
	readState() *model.BackupState

	// writeState writes the state object for the backup.
	writeState(*model.BackupState) error

	// writeBackupCreationTime writes creation time in the metadata file under the backup folder.
	writeBackupMetadata(path string, metadata model.BackupMetadata) error

	// CleanDir cleans the directory with the given name.
	CleanDir(name string) error

	// DeleteFolder removes file with a given path.
	DeleteFolder(path string) error
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
