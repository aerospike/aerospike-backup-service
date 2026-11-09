package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/models"
)

// BackupNamespacesOperation is the handle of a scan backup run: one backup-go handler per
// namespace, reported as a single run.
type BackupNamespacesOperation struct {
	handlers map[string]CancelableBackupHandler
}

var _ RoutineBackupHandler = (*BackupNamespacesOperation)(nil)

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

// Status sums the namespace handlers into one run, or nil while none of them has started.
// Progress is the share of the records estimated for all namespaces that are read so far: zero
// before the estimate is known, and capped at 1 because the estimate can fall short of the
// records actually read.
func (op *BackupNamespacesOperation) Status() *model.BackupRun {
	stats := op.aggregateStats()
	if stats == nil {
		return nil
	}

	total, done := stats.TotalRecords.Load(), stats.GetReadRecords()
	progress := 0.0
	if total != 0 {
		progress = min(float64(done)/float64(total), 1)
	}

	return &model.BackupRun{
		StartTime: stats.StartTime,
		Progress:  progress,
		Scan: &model.ScanProgress{
			TotalRecords: total,
			DoneRecords:  done,
			Metrics:      op.sumMetrics(),
		},
	}
}

// sumMetrics sums backup-go metrics across all namespace handlers.
func (op *BackupNamespacesOperation) sumMetrics() *models.Metrics {
	metrics := make([]*models.Metrics, 0, len(op.handlers))

	for _, handler := range op.handlers {
		metrics = append(metrics, handler.GetMetrics())
	}

	return models.SumMetrics(metrics...)
}

// aggregateStats sums the statistics of the namespace handlers that have started, or returns
// nil when none has.
func (op *BackupNamespacesOperation) aggregateStats() *models.BackupStats {
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
