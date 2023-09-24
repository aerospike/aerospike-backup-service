package service

import "github.com/aerospike/backup/pkg/model"

// BackupBackend represents a backup backend handler.
type BackupBackend interface {
	readState() *model.BackupState
	writeState(*model.BackupState) error
	backupList() ([]string, error)
}
