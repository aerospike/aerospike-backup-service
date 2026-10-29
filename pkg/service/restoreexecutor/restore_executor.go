package restoreexecutor

import (
	"context"
	"fmt"
	"slices"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// Restore starts a restore and returns a handle to observe it.
type Restore interface {
	// Run restores one backup, or a full backup and the incremental backups that follow it,
	// oldest first, one request per backup, and returns a single handler for all of them.
	Run(
		ctx context.Context,
		client aerospike.Client,
		full *model.RestoreRequest,
		incrementals ...*model.RestoreRequest,
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

// Run restores the backups one after another and returns at once: the restores run in the
// background, and the handler reports the stats of every restore started so far, and a
// failure to start one from Wait. Under the unique (CREATE_ONLY) policy a chain is applied
// newest first, so that a record keeps its latest version and the older ones are skipped,
// unless the request disables reordering; otherwise oldest first, so that later versions
// overwrite earlier ones.
func (r *RestoreExecutor) Run(
	ctx context.Context,
	client aerospike.Client,
	full *model.RestoreRequest,
	incrementals ...*model.RestoreRequest,
) (RestoreHandler, error) {
	requests := append([]*model.RestoreRequest{full}, incrementals...)
	if full.Policy.IsUniqueOrDefault() && !full.DisableReordering {
		slices.Reverse(requests)
	}

	chain := newSequentialRestoreHandler()

	go func() {
		chain.finish(r.runSequentially(ctx, client, requests, chain))
	}()

	return chain, nil
}

// runSequentially restores each request after the previous one has finished, and stops at
// the first failure.
func (r *RestoreExecutor) runSequentially(
	ctx context.Context,
	client aerospike.Client,
	requests []*model.RestoreRequest,
	chain *sequentialRestoreHandler,
) error {
	for _, request := range requests {
		handler, err := runScanRestore(ctx, client, request, r.operations)
		if err != nil {
			return fmt.Errorf("failed to start restore from backup at %s: %w", request.BackupDataPath, err)
		}

		chain.add(handler)

		if err := handler.Wait(ctx); err != nil {
			return err
		}
	}

	return nil
}
