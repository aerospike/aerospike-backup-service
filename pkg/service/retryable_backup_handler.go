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
		err := try.Retry(policy, logger.With(slog.String("label", "write metadata")), func() error {
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
		handler, err := callbacks.Start(ctx)
		if err != nil {
			return fmt.Errorf("failed to start backup: %w", err)
		}

		h.setHandler(handler)

		if err = handler.Wait(ctxWithCancel); err != nil {
			callbacks.OnFail(ctx)
			h.setHandler(nil)
			return fmt.Errorf("backup failed: %w", err)
		}

		return retryOnSuccess(handler)
	}

	// Start the backup process with retries
	go func() {
		h.errCh <- try.Retry(policy, logger.With(slog.String("label", "backup")), processBackup, callbacks.OnRetry)
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

func (h *retryableBackupHandler) Cancel() {
	h.Lock()
	defer h.Unlock()
	if h.cancel != nil {
		h.cancel()
	}
}
