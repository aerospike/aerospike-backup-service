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
func (b *BackupShared) BackupRun(_ *model.BackupRoutine, _ *model.BackupPolicy,
	_ *model.AerospikeCluster, _ *model.Storage, _ *model.SecretAgent,
	_ BackupOptions, _ *string, _ *string) (*BackupStat, error) {
	slog.Info("BackupRun mock call")
	return &BackupStat{}, nil
}
