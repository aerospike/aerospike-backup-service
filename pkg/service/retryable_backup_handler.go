package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/aerospike/backup-go/models"
)

type RetryableBackupHandler struct {
	sync.RWMutex
	handler BackupHandler
	errCh   chan error
}

func newRetryableBackupHandler(
	ctx context.Context,
	retry *RetryService,
	start func(ctx context.Context) (BackupHandler, error),
	onFail func(ctx context.Context),
	onSuccess func(ctx context.Context, stats *models.BackupStats) error,
) *RetryableBackupHandler {
	h := &RetryableBackupHandler{
		errCh: make(chan error, 1),
	}

	processBackup := func() error {
		handler, err := start(ctx)
		if err != nil {
			return fmt.Errorf("failed to start backup: %w", err)
		}

		// Store the handler safely
		h.Lock()
		h.handler = handler
		h.Unlock()

		if err = handler.Wait(ctx); err != nil {
			onFail(ctx)
			return fmt.Errorf("backup failed: %w", err)
		}

		return onSuccess(ctx, handler.GetStats())
	}

	// Start the backup process with retries
	go func() {
		h.errCh <- retry.retry(processBackup)
	}()

	return h
}

func (h *RetryableBackupHandler) Wait(ctx context.Context) error {
	select {
	case err := <-h.errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *RetryableBackupHandler) GetStats() *models.BackupStats {
	h.RLock()
	defer h.RUnlock()
	if h.handler != nil {
		return h.handler.GetStats()
	}
	return &models.BackupStats{}
}
