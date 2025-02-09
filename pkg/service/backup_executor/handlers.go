package backup_executor

import (
	"context"
	"errors"
	"fmt"

	"github.com/aerospike/backup-go/models"
)

// Backup handlers.
type SimpleBackupHandler struct {
	handler BackupHandler
}

func (h *SimpleBackupHandler) Wait(ctx context.Context) error {
	return h.handler.Wait(ctx)
}

func (h *SimpleBackupHandler) GetStats() *models.BackupStats {
	return h.handler.GetStats()
}

type CombinedBackupHandler struct {
	xdrHandler  BackupHandler
	scanHandler BackupHandler
}

func (h *CombinedBackupHandler) Wait(ctx context.Context) error {
	var errs []error

	if err := h.xdrHandler.Wait(ctx); err != nil {
		errs = append(errs, fmt.Errorf("XDR backup failed: %w", err))
	}
	if err := h.scanHandler.Wait(ctx); err != nil {
		errs = append(errs, fmt.Errorf("scan backup failed: %w", err))
	}

	return errors.Join(errs...)
}
func (h *CombinedBackupHandler) GetStats() *models.BackupStats {
	xdrStats := h.xdrHandler.GetStats()
	scanStats := h.scanHandler.GetStats()

	return &models.BackupStats{
		TotalRecords: xdrStats.TotalRecords + scanStats.TotalRecords,
		//TODO: add more stats here
	}
}
