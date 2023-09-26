package service

import "github.com/aerospike/backup/pkg/model"

// BackupBackend represents a backup backend handler.
type BackupBackend interface {

	// readState reads and returns the state for the backup.
	readState() *model.BackupState

	// writeState writes the state object for the backup.
	writeState(*model.BackupState) error

	// CleanDir cleans the directory with the given name.
	CleanDir(name string)

	// FullBackupList returns a list of available full backups.
	FullBackupList() ([]string, error)

	// IncrementalBackupList returns a list of available incremental backups.
	IncrementalBackupList() ([]string, error)

	// BackupPolicyName returns the name of the defining backup policy.
	BackupPolicyName() string
}
