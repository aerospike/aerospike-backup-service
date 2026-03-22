package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go/models"
)

// SingleNamespaceExecutor runs the backup pipeline for one namespace: retries,
// backend cleanup on failure/cancel, and metadata writes on success.
type SingleNamespaceExecutor interface {
	// Run starts backup execution for a single namespace and returns a cancelable handler.
	Run(
		ctx context.Context,
		logger *slog.Logger,
		backupRoutine *model.BackupRoutine,
		namespace string,
		runSpec model.BackupRunSpec,
	) CancelableBackupHandler
}

// NewSingleNamespaceExecutor builds a [SingleNamespaceExecutor] from the low-level
// backup runner, storage writer, and path layout service.
func NewSingleNamespaceExecutor(
	backupExecutor backupexecutor.Backup,
	backendService BackupWriter,
	pathService PathService,
) SingleNamespaceExecutor {
	return &singleNamespaceExecutorImpl{
		backupExecutor: backupExecutor,
		backendService: backendService,
		pathService:    pathService,
	}
}

type singleNamespaceExecutorImpl struct {
	backupExecutor backupexecutor.Backup
	backendService BackupWriter
	pathService    PathService
}

// CancelableBackupHandler is a [backupexecutor.BackupHandler] that can be canceled
// while a backup is in progress.
type CancelableBackupHandler interface {
	backupexecutor.BackupHandler
	// Cancel requests cancellation of the in-flight backup.
	Cancel()
}

// Run starts a retryable backup for one namespace via [backupexecutor.Backup.Run],
// with cleanup and metadata callbacks wired for the routine's storage layout.
func (e *singleNamespaceExecutorImpl) Run(
	ctx context.Context,
	logger *slog.Logger,
	backupRoutine *model.BackupRoutine,
	namespace string,
	runSpec model.BackupRunSpec,
) CancelableBackupHandler {
	backupFolder := e.pathService.GetBackupPath(backupRoutine.Name, runSpec.Type, namespace, runSpec.StartTime)

	return newRetryableBackupHandler(
		ctx,
		*backupRoutine.BackupPolicy.GetRetryPolicyOrDefault(),
		retryableBackupCallbacks{
			Start: func(ctx context.Context) (backupexecutor.BackupHandler, error) {
				return e.backupExecutor.Run(ctx, backupRoutine, runSpec.TimeBounds, namespace, backupFolder)
			},
			OnFail: func(ctx context.Context) {
				path := e.pathService.GetTimestampPath(backupRoutine.Name, runSpec.StartTime, runSpec.Type)
				e.deleteFolder(ctx, backupRoutine, path, logger)
			},
			OnSuccess: func(ctx context.Context, stats *models.BackupStats) error {
				if runSpec.Type == model.BackupTypeIncremental && stats.IsEmpty() {
					return nil
				}
				metadata := model.NewBackupMetadata(
					stats,
					namespace,
					ptr.ValueOrZero(runSpec.TimeBounds.FromTime),
					runSpec.StartTime,
					backupRoutine.BackupPolicy,
				)

				return e.writeBackupMetadata(ctx, logger, backupRoutine, metadata, backupFolder)
			},
			OnRetry: func() {
				observeBackupEvent(backupRoutine.Name, runSpec.Type, BackupOutcomeRetry, 0)
			},
		},
		logger,
	)
}

// deleteFolder removes backup data under the given path on failure or cancel; logs
// errors except when the context was canceled during delete.
func (e *singleNamespaceExecutorImpl) deleteFolder(
	ctx context.Context,
	routine *model.BackupRoutine,
	path string,
	logger *slog.Logger,
) {
	err := e.backendService.Delete(ctx, routine, path)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			logger.Info("Delete folder context canceled")
			return
		}

		logger.Error("Failed to delete folder", attr.Error(err))
		return
	}
}

// writeBackupMetadata persists backup metadata to storage and logs the folder on success.
func (e *singleNamespaceExecutorImpl) writeBackupMetadata(
	ctx context.Context,
	logger *slog.Logger,
	routine *model.BackupRoutine,
	metadata model.BackupMetadata,
	backupFolder string,
) error {
	if err := e.backendService.WriteBackupMetadata(ctx, routine, backupFolder, metadata); err != nil {
		return fmt.Errorf("failed to write backup metadata to %q: %w", backupFolder, err)
	}

	logger.Info("Wrote backup metadata",
		slog.Any("folder", backupFolder),
		slog.Any("metadata", metadata))

	return nil
}
