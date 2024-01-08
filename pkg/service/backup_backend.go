package service

import (
	"github.com/aerospike/backup/pkg/model"
	"time"
)

// BackupBackend represents a backup backend handler.
type BackupBackend interface {
	BackupListReader

	// readState reads and returns the state for the backup.
	readState() *model.BackupState

	// writeState writes the state object for the backup.
	writeState(*model.BackupState) error

	writeFullBackupCreationTime(path string, timestamp time.Time) error

	writeIncrementalBackupCreationTime(path string, timestamp time.Time) error

	readFullBackupCreationTime(path string) (time.Time, error)

	readIncrementalBackupCreationTime(path string) (time.Time, error)

	// CleanDir cleans the directory with the given name.
	CleanDir(name string) error

	// DeleteFile removes file with a given path.
	DeleteFile(path string) error
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
