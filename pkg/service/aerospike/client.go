package aerospike

import (
	"context"

	"github.com/aerospike/backup-go"
)

// Backuper exposes only backup-related methods.
type Backuper interface {
	// Backup starts a backup operation that writes data to a provided writer.
	Backup(
		ctx context.Context,
		config *backup.ConfigBackup,
		writer backup.Writer,
		reader backup.StreamingReader,
	) (*backup.BackupHandler, error)
	// InfoClient returns the underlying info client.
	InfoClient() backup.InfoGetter
}

// Restorer exposes only restore-related methods.
type Restorer interface {
	// Restore starts a restore operation that reads data from given readers.
	Restore(
		ctx context.Context,
		config *backup.ConfigRestore,
		streamingReader backup.StreamingReader,
	) (backup.Restorer, error)
	// InfoClient returns the underlying info client.
	InfoClient() backup.InfoGetter
}

// Client interface for backup.Client's public API.
type Client interface {
	Backuper
	Restorer

	// AerospikeClient returns the underlying Aerospike client.
	AerospikeClient() backup.AerospikeClient
}

var _ Client = (*backup.Client)(nil)
