package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// BackupRoutineStarter starts a backup operation for set of namespaces for given routine.
// It encapsulates all backup operations for the whole routine.
type BackupRoutineStarter struct {
	starter *BackupNamespaceRunner
}

// NewBackupRoutineStarter creates a new instance of BackupRoutineStarter.
// All parameters passed to this function are immutable and will not be changed
// during the lifecycle of the backup routine.
func NewBackupRoutineStarter(
	routineName string,
	backupService Backup,
	backupPolicy *model.BackupPolicy,
	retry executor,
	metadataWriter BackupMetadataWriter,
	jobType jobType,
	logger *slog.Logger,
) *BackupRoutineStarter {
	return &BackupRoutineStarter{
		starter: NewBackupNamespaceRunner(routineName, backupService, backupPolicy, retry, metadataWriter, jobType, logger),
	}
}

// Start initiates a new backup process for the specified namespaces.
// The parameters passed to this method are specific to each execution.
func (s *BackupRoutineStarter) Start(
	ctx context.Context,
	client *backup.Client,
	namespaces []string,
	timeBounds model.TimeBounds,
	now time.Time,
) *BackupNamespacesOperation {
	op := &BackupNamespacesOperation{
		handlers: make(map[string]CancelableBackupHandler, len(namespaces)),
	}

	for _, namespace := range namespaces {
		op.handlers[namespace] = s.starter.Run(ctx, client, namespace, now, timeBounds)
	}

	return op
}

// BackupNamespacesOperation orchestrates backup operations across multiple namespaces.
// It creates and manages individual BackupOperation instances for each namespace and
// waits for their execution.
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

	res := &models.BackupStats{}
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
