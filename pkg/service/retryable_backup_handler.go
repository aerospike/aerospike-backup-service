package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
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

func newRetryableBackupHandler(
	ctx context.Context,
	retry executor,
	start func(ctx context.Context) (backupexecutor.BackupHandler, error),
	onFail func(ctx context.Context),
	onSuccess func(ctx context.Context, stats *models.BackupStats) error,
	onRetry func(),
) *retryableBackupHandler {
	ctxWithCancel, cancel := context.WithCancel(ctx)
	h := &retryableBackupHandler{
		errCh:  make(chan error, 1),
		cancel: cancel,
	}

	// Helper to retry onSuccess only
	retryOnSuccess := func(handler backupexecutor.BackupHandler) error {
		err := retry.run("write metadata", func() error {
			return onSuccess(ctx, handler.GetStats())
		}, func() {})
		if err != nil {
			// Trigger onFail if onSuccess ultimately fails
			onFail(ctx)
		}

		return err
	}

	// Process backup function.
	processBackup := func() error {
		handler, err := start(ctx)
		if err != nil {
			return fmt.Errorf("failed to start backup: %w", err)
		}

		h.setHandler(handler)

		if err = handler.Wait(ctxWithCancel); err != nil {
			onFail(ctx)
			h.setHandler(nil)
			return fmt.Errorf("backup failed: %w", err)
		}

		return retryOnSuccess(handler)
	}

	// Start the backup process with retries
	go func() {
		h.errCh <- retry.run("backup", processBackup, onRetry)
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
