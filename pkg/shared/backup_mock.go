//go:build ci

package shared

import (
	"github.com/aerospike/backup/pkg/model"
	"log/slog"
)

// BackupShared mocks the Backup interface.
// Used in CI workflows to skip building the C shared libraries.
type BackupShared struct {
}

var _ Backup = (*BackupShared)(nil)

// NewBackup returns a new BackupShared instance.
func NewBackup() *BackupShared {
	return &BackupShared{}
}

// BackupRun mocks the interface method.
func (b *BackupShared) BackupRun(backupRoutine *model.BackupRoutine, backupPolicy *model.BackupPolicy,
	cluster *model.AerospikeCluster, storage *model.Storage, secretAgent *model.SecretAgent,
	opts BackupOptions, path *string) *BackupStat {
	slog.Info("BackupRun mock call")
	return &BackupStat{}
}
