package restoreexecutor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// Restore starts a restore and returns a handle to observe it.
type Restore interface {
	// Run restores the backups of the requests one after another, in the order given, one
	// request per backup, and returns a single handler for all of them. Every failure, including
	// one to start, comes from the handler's Wait. The start of each restore is logged as a step
	// of the sequence.
	Run(
		ctx context.Context,
		client aerospike.Client,
		logger *slog.Logger,
		requests ...*model.RestoreRequest,
	) RestoreHandler
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

// Run restores the requests in the background, each after the previous one has finished. The
// handler reports the stats of every restore started so far and, from Wait, the first failure.
// ctx bounds the whole sequence: canceling it stops the running restore and starts no more.
// The ctx given to Wait bounds only that wait.
func (r *RestoreExecutor) Run(
	ctx context.Context,
	client aerospike.Client,
	logger *slog.Logger,
	requests ...*model.RestoreRequest,
) RestoreHandler {
	chain := newSequentialRestoreHandler()

	go func() {
		chain.finish(r.restoreAll(ctx, client, logger, requests, chain))
	}()

	return chain
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
