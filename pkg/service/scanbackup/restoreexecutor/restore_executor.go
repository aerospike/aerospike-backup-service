package restoreexecutor

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// RestoreExecutor runs scan restores with backup-go, reading the data through storage operations.
type RestoreExecutor struct {
	operations storage.Operations
}

// NewRestoreExecutor returns a RestoreExecutor.
func NewRestoreExecutor(operations storage.Operations) *RestoreExecutor {
	return &RestoreExecutor{
		operations: operations,
	}
}

// Run starts the restore of one backup and returns the handler to follow it.
func (r *RestoreExecutor) Run(
	ctx context.Context,
	client aerospike.Client,
	request *model.RestoreRequest,
) (service.RestoreHandler, error) {
	return runScanRestore(ctx, client, request, r.operations)
}
