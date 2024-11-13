package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/aerospike/backup-go/models"
)

type retryableBackupHandler struct {
	sync.RWMutex
	handler BackupHandler
	errCh   chan error
}

var _ BackupHandler = (*retryableBackupHandler)(nil)

func startBackup(
	ctx context.Context,
	retry Executor,
	start func(ctx context.Context) (BackupHandler, error),
	onFail func(ctx context.Context),
	onSuccess func(ctx context.Context, stats *models.BackupStats) error,
) BackupHandler {
	h := &retryableBackupHandler{
		errCh: make(chan error, 1),
	}

	// Helper to retry onSuccess only
	retryOnSuccess := func(handler BackupHandler) error {
		err := retry.run("write metadata", func() error {
			return onSuccess(ctx, handler.GetStats())
		})
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

		if err = handler.Wait(ctx); err != nil {
			onFail(ctx)
			h.setHandler(nil)
			return fmt.Errorf("backup failed: %w", err)
		}

		return retryOnSuccess(handler)
	}

	// Start the backup process with retries
	go func() {
		h.errCh <- retry.run("backup", processBackup)
	}()

	return h
}

func (h *retryableBackupHandler) setHandler(handler BackupHandler) {
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
