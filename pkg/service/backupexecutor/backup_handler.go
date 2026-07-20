package backupexecutor

import (
	"context"
	"errors"
	"fmt"

	"github.com/aerospike/backup-go/models"
)

// BackupHandler interface defines the contract for backup operation results.
type BackupHandler interface {
	// GetStats returns backup job statistics.
	//
	// Nil means the job has not started yet. Wrappers (e.g. retryableBackupHandler)
	// return nil until their inner pipeline is running. backup-go handlers never
	// return nil once constructed.
	GetStats() *models.BackupStats
	// Wait waits for the backup job to complete and returns an error if the job failed.
	Wait(context.Context) error
	// GetMetrics returns the performance metrics of the backup job.
	GetMetrics() *models.Metrics
}

// CombinedBackupHandler is a wrapper around two backup handlers.
// It combines the stats and waits on both handlers.
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
func (h *CombinedBackupHandler) GetMetrics() *models.Metrics {
	return models.SumMetrics(h.xdrHandler.GetMetrics(), h.scanHandler.GetMetrics())
}
