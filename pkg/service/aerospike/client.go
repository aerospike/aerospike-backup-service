package aerospike

import (
	"context"

	"github.com/aerospike/backup-go"
)

// Client is the backup-go client API this service uses: backup, restore, and cluster info.
type Client interface {
	// Backup starts a backup operation that writes data to a provided writer.
	Backup(
		ctx context.Context,
		config *backup.ConfigBackup,
		writer backup.Writer,
		reader backup.StreamingReader,
	) (backup.BackupHandler, error)
	// Restore starts a restore operation that reads data from given readers.
	Restore(
		ctx context.Context,
		config *backup.ConfigRestore,
		streamingReader backup.StreamingReader,
	) (backup.RestoreHandler, error)
	// InfoClient returns the underlying info client.
	InfoClient() backup.InfoGetter
	// AerospikeClient returns the underlying Aerospike client.
	AerospikeClient() backup.AerospikeClient
}

var _ Client = (*backup.Client)(nil)
