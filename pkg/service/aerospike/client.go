package aerospike

import (
	"context"

	"github.com/aerospike/backup-go"
)

// Backuper exposes only backup-related methods.
type Backuper interface {
	Backup(
		ctx context.Context,
		config *backup.ConfigBackup,
		writer backup.Writer,
		reader backup.StreamingReader,
	) (*backup.BackupHandler, error)
	BackupXDR(
		ctx context.Context,
		config *backup.ConfigBackupXDR,
		writer backup.Writer,
	) (*backup.HandlerBackupXDR, error)
	InfoClient() backup.InfoGetter
}

// Restorer exposes only restore-related methods.
type Restorer interface {
	Restore(
		ctx context.Context,
		config *backup.ConfigRestore,
		streamingReader backup.StreamingReader,
	) (backup.Restorer, error)
	InfoClient() backup.InfoGetter
}

// Client combines all sub-interfaces for the Client's public API.
type Client interface {
	Backuper
	Restorer

	AerospikeClient() backup.AerospikeClient
}

var _ Client = (*backup.Client)(nil)
