package backupexecutor

import (
	"context"
	"errors"
	"fmt"

	"github.com/aerospike/backup-go/models"
)

// BackupHandler interface defines the contract for backup operation results.
type BackupHandler interface {
	GetStats() *models.BackupStats
	Wait(context.Context) error
}

type CombinedBackupHandler struct {
	xdrHandler  BackupHandler
	scanHandler BackupHandler
}

var _ BackupHandler = (*CombinedBackupHandler)(nil)

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
	return models.SumBackupStats(h.xdrHandler.GetStats(), h.scanHandler.GetStats())
}
