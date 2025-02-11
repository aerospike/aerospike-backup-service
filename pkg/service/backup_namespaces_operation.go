package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// startNamespacesBackup initiates a new backup process for the routine (multiple namespaces).
// Each namespace is backed up independently using the provided BackupNamespaceRunner.
// Returns a BackupNamespacesOperation that tracks the progress and status of the backup processes.
func startNamespacesBackup(
	ctx context.Context,
	runner *BackupNamespaceRunner,
	client *backup.Client,
	namespaces []string,
	timeBounds model.TimeBounds,
	now time.Time,
	backupRoutine *model.BackupRoutine,
	jobType jobType,
) *BackupNamespacesOperation {
	op := &BackupNamespacesOperation{
		handlers: make(map[string]CancelableBackupHandler, len(namespaces)),
	}

	for _, namespace := range namespaces {
		op.handlers[namespace] = runner.Run(ctx, client, backupRoutine, jobType, namespace, now, timeBounds)
	}

	return op
}

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

// GetStats return aggregated public statistics for all inner handlers.
// return nil if no handlers are currently running.
func (op *BackupNamespacesOperation) GetStats() *models.BackupStats {
	activeHandlers := 0

	res := models.NewBackupStats()
	for _, handler := range op.handlers {
		if handler.GetStats() == nil {
			continue
		}

		activeHandlers++
		res.TotalRecords += handler.GetStats().TotalRecords
		res.ReadRecords.Add(handler.GetStats().GetReadRecords())
		res.BytesWritten.Add(handler.GetStats().BytesWritten.Load())

		// These are the backups of multiple namespaces in the same routine.
		// Therefore, picking any of those is valid, since they started at
		// the same time.
		res.StartTime = handler.GetStats().StartTime
	}

	if activeHandlers == 0 {
		return nil
	}

	return res
}
