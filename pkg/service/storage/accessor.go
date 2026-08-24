package storage

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/storage/options"
)

// Accessor opens backup-go readers and writers for one kind of storage backend
// (local, S3, GCP, Azure). [Operations] picks the accessor that supports a given storage.
type Accessor interface {
	supports(storage model.Storage) bool

	createReader(ctx context.Context, storage model.Storage, opts ...options.Opt) (backup.StreamingReader, error)

	createWriter(ctx context.Context, storage model.Storage, opts ...options.Opt) (backup.Writer, error)
}
