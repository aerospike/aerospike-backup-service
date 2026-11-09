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

// Status sums the namespace handlers that have started into one run, or returns nil while none
// has. A namespace waiting to retry has no stats, so it drops out until its next attempt starts.
// Progress is the share of the estimated records that are read so far: zero before the estimate
// is known, and capped at 1 because the estimate can fall short of the records actually read.
func (op *BackupNamespacesOperation) Status() *model.BackupRun {
	var total, done uint64
	started := false
	run := model.BackupRun{}
	metrics := make([]*models.Metrics, 0, len(op.handlers))
	for _, handler := range op.handlers {
		stats := handler.GetStats()
		if stats == nil {
			continue
		}

		started = true
		total += stats.TotalRecords.Load()
		done += stats.GetReadRecords()
		metrics = append(metrics, handler.GetMetrics())
		// The namespaces of a routine start together, so any one's start time is the run's.
		run.StartTime = stats.StartTime
	}

	if !started {
		return nil
	}

	if total != 0 {
		run.Progress = min(float64(done)/float64(total), 1)
	}

	run.Scan = &model.ScanProgress{
		TotalRecords: total,
		DoneRecords:  done,
		Metrics:      models.SumMetrics(metrics...),
	}

	return &run
}
