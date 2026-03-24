package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/aerospike/backup-go/models"
)

// BackupNamespacesOperation aggregates backup operations across multiple namespaces.
// It creates individual backup process instances for each namespace and
// waits for their execution.
// Implements CancelableBackupHandler, so it can be treated as a unified handler externally.
type BackupNamespacesOperation struct {
	handlers map[string]CancelableBackupHandler
}

var _ CancelableBackupHandler = (*BackupNamespacesOperation)(nil)

// Wait waits for all backup operations to complete and collects any errors
// that occurred during the backup process.
func (op *BackupNamespacesOperation) Wait(ctx context.Context) error {
	var aggregatedErr error
	for ns, handler := range op.handlers {
		if err := handler.Wait(ctx); err != nil {
			aggregatedErr = errors.Join(aggregatedErr, fmt.Errorf("namespace %s: %w", ns, err))
		}
	}
	return aggregatedErr
}

// Cancel stops all running backup operations.
func (op *BackupNamespacesOperation) Cancel() {
	for _, handler := range op.handlers {
		handler.Cancel()
	}
}

// GetMetrics sums backup-go metrics across all namespace handlers.
func (op *BackupNamespacesOperation) GetMetrics() *models.Metrics {
	metrics := make([]*models.Metrics, 0, len(op.handlers))

	for _, handler := range op.handlers {
		metrics = append(metrics, handler.GetMetrics())
	}

	return models.SumMetrics(metrics...)
}

// GetStats return aggregated public statistics for all inner handlers.
// return nil if no handlers are currently running.
func (op *BackupNamespacesOperation) GetStats() *models.BackupStats {
	activeHandlers := 0

	res := models.NewBackupStats()
	for _, handler := range op.handlers {
		backupStats := handler.GetStats()
		if backupStats == nil {
			continue
		}

		activeHandlers++
		res.TotalRecords.Add(backupStats.TotalRecords.Load())
		res.ReadRecords.Add(backupStats.GetReadRecords())
		res.BytesWritten.Add(backupStats.BytesWritten.Load())

		// These are the backups of multiple namespaces in the same routine.
		// Therefore, picking any of those is valid, since they started at
		// the same time.
		res.StartTime = backupStats.StartTime
	}

	if activeHandlers == 0 {
		return nil
	}

	return res
}
