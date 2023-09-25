package service

import "github.com/aerospike/backup/pkg/model"

// BackupBackend represents a backup backend handler.
type BackupBackend interface {

	// readState reads and returns the state for the backup.
	readState() *model.BackupState

	// writeState writes the state object for the backup.
	writeState(*model.BackupState) error

	// BackupList returns a list of available backups.
	BackupList() ([]string, error)

	// BackupPolicyName returns the name of the defining backup policy.
	BackupPolicyName() string
}
