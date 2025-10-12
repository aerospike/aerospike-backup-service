package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go/models"
)

// BackupNamespaceRunner starts a backup operation for a single namespace. It encapsulates
// all the logic needed to perform a backup of one namespace, including running the backup,
// managing metadata, and handling cleanup.
// Every routine has its own BackupNamespaceRunner.
type BackupNamespaceRunner struct {
	routineName    string
	backupExecutor backupexecutor.Backup
	retry          executor
	backendService BackupWriter
	logger         *slog.Logger
}

// NewBackupNamespaceRunner creates a new BackupNamespaceRunner instance.
func NewBackupNamespaceRunner(
	routineName string,
	backupExecutor backupexecutor.Backup,
	retry executor,
	backendService BackupWriter,
	logger *slog.Logger,
) *BackupNamespaceRunner {
	return &BackupNamespaceRunner{
		routineName:    routineName,
		backupExecutor: backupExecutor,
		retry:          retry,
		backendService: backendService,
		logger:         logger,
	}
}

// CancelableBackupHandler extends BackupHandler with support for canceling the backup.
type CancelableBackupHandler interface {
	backupexecutor.BackupHandler
	// Cancel cancels the backup operation.
	Cancel()
}

// Run executes the backup operation for the namespace. It handles the entire backup process
// including folder management, metadata writing, and error handling.
func (op *BackupNamespaceRunner) Run(
	ctx context.Context,
	client aerospike.Backuper,
	backupRoutine *model.BackupRoutine,
	backupType jobType,
	namespace string,
	startTime time.Time,
	timeBounds model.TimeBounds,
) CancelableBackupHandler {
	backupFolder := getBackupPath(op.routineName, backupType, namespace, startTime)

	return newRetryableBackupHandler(
		ctx,
		op.retry,
		func(ctx context.Context) (backupexecutor.BackupHandler, error) { // start
			return op.backupExecutor.Run(ctx, client, backupRoutine, timeBounds, namespace, backupFolder)
		},
		func(ctx context.Context) { // on fail
			op.deleteFolder(ctx, getTimestampPath(op.routineName, startTime, backupType))
		},
		func(ctx context.Context, stats *models.BackupStats) error { // on success
			// For incremental backups, skip metadata for empty backups
			if backupType == jobTypeIncremental && stats.IsEmpty() {
				return nil
			}
			metadata := model.NewBackupMetadata(
				stats, namespace, ptr.ValueOrZero(timeBounds.FromTime), startTime, backupRoutine.BackupPolicy,
			)
			return op.writeBackupMetadata(ctx, metadata, backupFolder)
		},
		func() { // on retry
			observeBackupEvent(op.routineName, backupType, BackupOutcomeRetry, 0)
		},
	)
}

func (op *BackupNamespaceRunner) deleteFolder(ctx context.Context, path string) {
	err := op.backendService.Delete(ctx, op.routineName, path)
	if err != nil {
		op.logger.Error("Could not delete folder", attr.Error(err))
		return
	}
}

func (op *BackupNamespaceRunner) writeBackupMetadata(
	ctx context.Context, metadata model.BackupMetadata, backupFolder string,
) error {
	if err := op.backendService.WriteBackupMetadata(ctx, op.routineName, backupFolder, metadata); err != nil {
		return fmt.Errorf("could not write backup metadata to %q: %w", backupFolder, err)
	}

	op.logger.Info("Wrote backup metadata",
		slog.Any("folder", backupFolder),
		slog.Any("metadata", metadata))

	return nil
}
