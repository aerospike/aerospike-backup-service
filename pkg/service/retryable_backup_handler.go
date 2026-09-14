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

// retryableBackupHandler is a wrapper around BackupHandler that adds
// retry logic and cancellation support.
//
// Two contexts take part in a run. The run context is the one the caller passes in: it comes
// from the scheduler, lives as long as the service does, and is what the backup pipeline is
// started with and what the failure and success callbacks run on. The wait context is derived
// from it and is this run's cancel handle: the retry loop and every Wait on the inner handler
// observe it, so Cancel ends both without touching the run context. Ending the wait context
// alone is enough to stop the pipeline, because the inner handler cancels its own work when the
// context it is waiting under ends, and it leaves the run context alive for the cleanup callback
// to delete the partial backup. The goroutine releases the wait context when the retry loop
// returns, whatever the outcome: a child context stays registered in its parent until it is
// canceled, and the parent here outlives every run.
type retryableBackupHandler struct {
	sync.RWMutex
	handler backupexecutor.BackupHandler
	cancel  context.CancelFunc
	errCh   chan error
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
	ctxWithCancel, cancel := context.WithCancel(ctx)
	h := &retryableBackupHandler{
		errCh:  make(chan error, 1),
		cancel: cancel,
	}

	// Helper to retry onSuccess only
	retryOnSuccess := func(handler backupexecutor.BackupHandler) error {
		err := try.Retry(ctx, policy, logger.With(slog.String("label", "write metadata")), func() error {
			return callbacks.OnSuccess(ctx, handler.GetStats())
		}, func() {})
		if err != nil {
			// Trigger onFail if onSuccess ultimately fails
			callbacks.OnFail(ctx)
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
			// The run context is still alive after Cancel, so the cleanup can reach storage.
			callbacks.OnFail(ctx)
			h.setHandler(nil)
			return fmt.Errorf("backup failed: %w", err)
		}

		return retryOnSuccess(handler)
	}

	// Start the backup process with retries. The wait context is released as soon as the loop
	// returns, so a finished run leaves nothing behind in the scheduler context.
	go func() {
		defer cancel()
		h.errCh <- try.Retry(ctxWithCancel, policy,
			logger.With(slog.String("label", "backup")), processBackup, callbacks.OnRetry)
	}()

	return h
}

func (h *retryableBackupHandler) setHandler(handler backupexecutor.BackupHandler) {
	h.Lock()
	defer h.Unlock()
	h.handler = handler
}

func (h *retryableBackupHandler) Wait(ctx context.Context) error {
	select {
	case err := <-h.errCh:
		return err
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
