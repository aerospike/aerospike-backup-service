package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/try"
	"github.com/aerospike/backup-go/models"
)

// retryableBackupHandler wraps a BackupHandler with retries and cancellation.
type retryableBackupHandler struct {
	sync.RWMutex
	handler backupexecutor.BackupHandler
	cancel  context.CancelFunc
	// done is closed once the retry loop has returned; err holds its outcome and is only
	// read after that.
	done chan struct{}
	err  error
}

var _ backupexecutor.BackupHandler = (*retryableBackupHandler)(nil)

// retryableBackupCallbacks configures the backup lifecycle hooks for newRetryableBackupHandler.
type retryableBackupCallbacks struct {
	Start     func(ctx context.Context) (backupexecutor.BackupHandler, error)
	OnFail    func(ctx context.Context)
	OnSuccess func(ctx context.Context, stats *models.BackupStats) error
	OnRetry   func()
}

func newRetryableBackupHandler(
	ctx context.Context,
	policy models.RetryPolicy,
	callbacks retryableBackupCallbacks,
	logger *slog.Logger,
) *retryableBackupHandler {
	// A run uses three contexts: the caller's, which starts the pipeline and outlives every run; a
	// cancelable child of it, which is this run's handle and what Cancel ends; and a cleanup context
	// that survives cancellation. Ending the child stops the pipeline, because the inner handler
	// cancels its own work when the context it waits under ends.
	ctxWithCancel, cancel := context.WithCancel(ctx)
	// Cleanup must still run when the run itself was canceled (shutdown, user cancel), otherwise
	// the partial backup folder stays in storage.
	cleanupCtx := context.WithoutCancel(ctx)
	h := &retryableBackupHandler{
		done:   make(chan struct{}),
		cancel: cancel,
	}

	// Helper to retry onSuccess only. The loop observes the wait context, so Cancel stops further
	// attempts; the write itself runs on the run context and is left to finish once started.
	retryOnSuccess := func(handler backupexecutor.BackupHandler) error {
		err := try.Retry(ctxWithCancel, policy, logger.With(slog.String("label", "write metadata")), func() error {
			return callbacks.OnSuccess(ctx, handler.GetStats())
		}, func() {})
		if err != nil {
			// Trigger onFail if onSuccess ultimately fails
			callbacks.OnFail(cleanupCtx)
		}

		return err
	}

	// Process backup function.
	processBackup := func() error {
		// The pipeline is started on the run context; only the wait is tied to this run's handle.
		handler, err := callbacks.Start(ctx)
		if err != nil {
			return fmt.Errorf("failed to start backup: %w", err)
		}

		h.setHandler(handler)

		if err = handler.Wait(ctxWithCancel); err != nil {
			callbacks.OnFail(cleanupCtx)
			h.setHandler(nil)
			return fmt.Errorf("backup failed: %w", err)
		}

		return retryOnSuccess(handler)
	}

	// Start the backup process with retries. The wait context is released as soon as the loop
	// returns, so a finished run leaves nothing behind in the scheduler context.
	go func() {
		defer cancel()

		h.finish(try.Retry(ctxWithCancel, policy,
			logger.With(slog.String("label", "backup")), processBackup, callbacks.OnRetry))
	}()

	return h
}

func (h *retryableBackupHandler) setHandler(handler backupexecutor.BackupHandler) {
	h.Lock()
	defer h.Unlock()
	h.handler = handler
}

// finish records the run's outcome and releases everyone waiting on it.
func (h *retryableBackupHandler) finish(err error) {
	h.Lock()
	h.err = err
	h.Unlock()

	close(h.done)
}

// Done is closed once the run has finished, successfully or not. A run whose pipeline never
// started ends here with GetStats still nil, so a caller waiting for the start has to watch it.
func (h *retryableBackupHandler) Done() <-chan struct{} {
	return h.done
}

// Err returns the run's outcome. It is meaningful only once Done is closed.
func (h *retryableBackupHandler) Err() error {
	h.RLock()
	defer h.RUnlock()

	return h.err
}

func (h *retryableBackupHandler) Wait(ctx context.Context) error {
	select {
	case <-h.done:
		return h.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetStats returns backup statistics from the inner handler, or nil before Start() completes.
func (h *retryableBackupHandler) GetStats() *models.BackupStats {
	h.RLock()
	defer h.RUnlock()
	if h.handler != nil {
		return h.handler.GetStats()
	}

	return nil
}

func (h *retryableBackupHandler) GetMetrics() *models.Metrics {
	h.RLock()
	defer h.RUnlock()
	if h.handler != nil {
		return h.handler.GetMetrics()
	}

	return nil
}

// Cancel ends this run's wait context, which stops the inner handler and the retry loop. The
// same function runs when the loop returns on its own, and a CancelFunc is idempotent, so Cancel
// is safe at any time and any number of times.
func (h *retryableBackupHandler) Cancel() {
	h.cancel()
}
