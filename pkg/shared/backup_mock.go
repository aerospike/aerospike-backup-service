//go:build ci

package shared

import (
	"log/slog"

	"github.com/aerospike/backup/pkg/model"
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
func (b *BackupShared) BackupRun(backupPolicy *model.BackupPolicy, cluster *model.AerospikeCluster,
	storage *model.BackupStorage, opts BackupOptions) bool {
	slog.Info("BackupRun mock call")
	return true
}
