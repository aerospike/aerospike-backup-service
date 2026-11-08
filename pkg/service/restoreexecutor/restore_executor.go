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
	// oldest first, one request per backup, and returns a single handler for all of them. The
	// policy and options of the full request stand for the whole chain. A chain whose first
	// restore cannot start is the error of Run; a later one fails from the handler's Wait.
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

// Run starts the first restore and returns; the rest of the chain runs in the background,
// each restore after the previous one has finished. The handler reports the stats of every
// restore started so far and, from Wait, the first failure. Under the unique (CREATE_ONLY)
// policy a chain is applied newest first, so that a record keeps its latest version and the
// older ones are skipped, unless the request disables reordering; otherwise oldest first, so
// that later versions overwrite earlier ones.
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

	first, err := r.startRestore(ctx, client, requests[0])
	if err != nil {
		return nil, err
	}

	chain := newSequentialRestoreHandler(first)

	go func() {
		chain.finish(r.runRest(ctx, client, first, requests[1:], chain))
	}()

	return chain, nil
}

// runRest waits for the first restore, then restores the rest one after another, stopping at
// the first failure.
func (r *RestoreExecutor) runRest(
	ctx context.Context,
	client aerospike.Client,
	first RestoreHandler,
	rest []*model.RestoreRequest,
	chain *sequentialRestoreHandler,
) error {
	if err := first.Wait(ctx); err != nil {
		return err
	}

	for _, request := range rest {
		handler, err := r.startRestore(ctx, client, request)
		if err != nil {
			return err
		}

		chain.add(handler)

		if err := handler.Wait(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (r *RestoreExecutor) startRestore(
	ctx context.Context,
	client aerospike.Client,
	request *model.RestoreRequest,
) (RestoreHandler, error) {
	handler, err := runScanRestore(ctx, client, request, r.operations)
	if err != nil {
		return nil, fmt.Errorf("failed to start restore from backup at %s: %w", request.BackupDataPath, err)
	}

	return handler, nil
}
