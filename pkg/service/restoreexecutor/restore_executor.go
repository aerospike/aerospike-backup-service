package restoreexecutor

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/storage/options"
)

type storageReader interface {
	// CreateDirReader creates a reader for a folder in the specified storage.
	CreateDirReader(ctx context.Context, storage model.Storage, path string, opts ...options.Opt,
	) (backup.StreamingReader, error)
}

// Restore represents a restore service.
type Restore interface {
	// Run executes the restore process and returns a restore handler for monitoring progress.
	Run(
		ctx context.Context,
		client aerospike.Client,
		restoreRequest *model.RestoreRequest,
	) (RestoreHandler, error)
}

// RestoreExecutor implements the [Restore] interface.
type RestoreExecutor struct {
	operations storageReader
}

// NewRestoreExecutor returns a new RestoreExecutor instance.
func NewRestoreExecutor(operations storageReader) *RestoreExecutor {
	return &RestoreExecutor{
		operations: operations,
	}
}

// Run initiates the restore operation.
// A restore handler is returned to monitor the job status.
func (r *RestoreExecutor) Run(
	ctx context.Context,
	client aerospike.Client,
	request *model.RestoreRequest,
) (RestoreHandler, error) {
	return runScanRestore(ctx, client, request, r.operations)
}
