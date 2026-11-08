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
	// Run restores one backup, or several one after another in the order given, one request
	// per backup, and returns a single handler for all of them. A first restore that cannot
	// start is the error of Run; a later one fails from the handler's Wait. The start of each
	// restore is logged as a step of the sequence.
	Run(
		ctx context.Context,
		client aerospike.Client,
		logger *slog.Logger,
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
// and, from Wait, the first failure. ctx bounds the whole sequence: canceling it stops the
// running restore and starts no more. The ctx given to Wait bounds only that wait.
func (r *RestoreExecutor) Run(
	ctx context.Context,
	client aerospike.Client,
	logger *slog.Logger,
	first *model.RestoreRequest,
	rest ...*model.RestoreRequest,
) (RestoreHandler, error) {
	steps := 1 + len(rest)

	step, err := r.start(ctx, client, logger.With(stepAttr(1, steps)), first)
	if err != nil {
		return nil, err
	}

	chain := newSequentialRestoreHandler(step.handler)

	go func() {
		chain.finish(r.runRest(ctx, client, logger, step, rest, chain))
	}()

	return chain, nil
}

// runRest waits for the first restore, then restores the rest one after another, stopping at
// the first failure.
func (r *RestoreExecutor) runRest(
	ctx context.Context,
	client aerospike.Client,
	logger *slog.Logger,
	first restoreStep,
	rest []*model.RestoreRequest,
	chain *sequentialRestoreHandler,
) error {
	if err := first.wait(ctx); err != nil {
		return err
	}

	steps := 1 + len(rest)
	for i, request := range rest {
		// A step can finish just as ctx is canceled; don't start the next one then.
		if err := ctx.Err(); err != nil {
			return err
		}

		step, err := r.start(ctx, client, logger.With(stepAttr(i+2, steps)), request)
		if err != nil {
			return err
		}

		chain.add(step.handler)

		if err := step.wait(ctx); err != nil {
			return err
		}
	}

	return nil
}

// restoreStep is a restore of one backup that has started.
type restoreStep struct {
	request *model.RestoreRequest
	handler RestoreHandler
}

// start logs and starts the restore of one backup. Only the start of a step is logged: the
// caller logs the outcome of the whole sequence once.
func (r *RestoreExecutor) start(
	ctx context.Context,
	client aerospike.Client,
	logger *slog.Logger,
	request *model.RestoreRequest,
) (restoreStep, error) {
	logger.Info("Start restoring", slog.String("path", request.BackupDataPath))

	handler, err := runScanRestore(ctx, client, request, r.operations)
	if err != nil {
		return restoreStep{}, fmt.Errorf("failed to start restore from backup at %s: %w",
			request.BackupDataPath, err)
	}

	return restoreStep{request: request, handler: handler}, nil
}

// wait waits for the restore to finish. A failure names the backup, which the restore's own
// error may not.
func (s restoreStep) wait(ctx context.Context) error {
	if err := s.handler.Wait(ctx); err != nil {
		return fmt.Errorf("failed to restore backup at %s: %w", s.request.BackupDataPath, err)
	}

	return nil
}

// stepAttr is the position of a restore in its sequence, as "i/n".
func stepAttr(step, steps int) slog.Attr {
	return slog.String("step", fmt.Sprintf("%d/%d", step, steps))
}
