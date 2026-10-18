package restoreexecutor

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// Restore starts a restore from one backup location and returns a handle to observe it.
type Restore interface {
	// Run executes the restore process and returns a restore handler for monitoring progress.
	Run(
		ctx context.Context,
		client aerospike.Client,
		restoreRequest *model.RestoreRequest,
	) (RestoreHandler, error)
}

// RestoreExecutor runs scan restores with backup-go, reading the data through storage operations.
type RestoreExecutor struct {
	operations storage.Operations
}

var _ Restore = (*RestoreExecutor)(nil)

// NewRestoreExecutor returns a RestoreExecutor.
func NewRestoreExecutor(operations storage.Operations) *RestoreExecutor {
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
