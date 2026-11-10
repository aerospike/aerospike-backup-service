package restoreexecutor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// RestoreExecutor runs scan restores with backup-go, reading the data through storage operations.
type RestoreExecutor struct {
	operations storage.Operations
}

var _ service.Restore = (*RestoreExecutor)(nil)

// NewRestoreExecutor returns a RestoreExecutor.
func NewRestoreExecutor(operations storage.Operations) *RestoreExecutor {
	return &RestoreExecutor{
		operations: operations,
	}
}

// errStoppedEarly is the outcome of a sequence whose goroutine ended before restoreAll returned.
var errStoppedEarly = errors.New("restore sequence stopped before it finished")

// Run restores the requests in the background, in the order inRestoreOrder gives, each after the
// previous one has finished. The handler reports the stats of every restore started so far and,
// from Wait, the first failure. Canceling ctx, or the one given to Wait, stops the running restore
// and starts no more.
func (r *RestoreExecutor) Run(
	ctx context.Context,
	client aerospike.Client,
	logger *slog.Logger,
	requests ...*model.RestoreRequest,
) service.RestoreHandler {
	ctx, cancel := context.WithCancel(ctx)
	chain := newSequentialRestoreHandler(cancel)

	go func() {
		// Deferred, so that Wait returns even if restoreAll leaves this goroutine early, as a
		// failed test expectation does through runtime.Goexit.
		err := errStoppedEarly
		defer func() {
			cancel()
			chain.finish(err)
		}()

		err = r.restoreChain(ctx, client, logger, requests, chain)
	}()

	return chain
}

// restoreChain restores the requests in the order inRestoreOrder gives.
func (r *RestoreExecutor) restoreChain(
	ctx context.Context,
	client aerospike.Client,
	logger *slog.Logger,
	requests []*model.RestoreRequest,
	chain *sequentialRestoreHandler,
) error {
	ordered, err := inRestoreOrder(ctx, client, logger, requests)
	if err != nil {
		return err
	}

	return r.restoreAll(ctx, client, logger, ordered, chain)
}

// restoreAll restores the requests one after another, stopping at the first failure. Only the
// start of each restore is logged: the caller logs the outcome of the whole sequence once. A
// failure names the backup, which the restore's own error may not.
func (r *RestoreExecutor) restoreAll(
	ctx context.Context,
	client aerospike.Client,
	logger *slog.Logger,
	requests []*model.RestoreRequest,
	chain *sequentialRestoreHandler,
) error {
	for i, request := range requests {
		// A restore can finish just as ctx is canceled; don't start the next one then.
		if err := ctx.Err(); err != nil {
			return err
		}

		path := request.BackupDataPath
		logger.Info("Start restoring",
			slog.String("step", fmt.Sprintf("%d/%d", i+1, len(requests))), slog.String("path", path))

		handler, err := runScanRestore(ctx, client, request, r.operations)
		if err != nil {
			return fmt.Errorf("failed to start restore from backup at %s: %w", path, err)
		}

		chain.add(handler)

		if err := handler.Wait(ctx); err != nil {
			return fmt.Errorf("failed to restore backup at %s: %w", path, err)
		}
	}

	return nil
}
