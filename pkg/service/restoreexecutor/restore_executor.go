package restoreexecutor

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// Restore starts a restore and returns a handle to observe it.
type Restore interface {
	// Run restores one backup, or several one after another in the order given, one request
	// per backup, and returns a single handler for all of them. A first restore that cannot
	// start is the error of Run; a later one fails from the handler's Wait.
	Run(
		ctx context.Context,
		client aerospike.Client,
		first *model.RestoreRequest,
		rest ...*model.RestoreRequest,
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

// Run starts the first restore and returns; the rest run in the background, each after the
// previous one has finished. The handler reports the stats of every restore started so far
// and, from Wait, the first failure.
func (r *RestoreExecutor) Run(
	ctx context.Context,
	client aerospike.Client,
	first *model.RestoreRequest,
	rest ...*model.RestoreRequest,
) (RestoreHandler, error) {
	handler, err := r.startRestore(ctx, client, first)
	if err != nil {
		return nil, err
	}

	chain := newSequentialRestoreHandler(handler)

	go func() {
		chain.finish(r.runRest(ctx, client, handler, rest, chain))
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
