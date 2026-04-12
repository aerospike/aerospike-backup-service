package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/prometheus"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go/models"
)

// NamespaceBackupRunner runs the backup pipeline for one namespace: retries,
// backend cleanup on failure/cancel, and metadata writes on success.
type NamespaceBackupRunner interface {
	// Run starts backup execution for a single namespace and returns a cancelable handler.
	Run(
		ctx context.Context,
		routine *model.BackupRoutine,
		namespace string,
		runSpec model.BackupRunSpec,
		logger *slog.Logger,
	) CancelableBackupHandler
}

// NewNamespaceBackupRunner builds a [NamespaceBackupRunner] from the low-level
// backup routineRunner, storage writer, and path layout service.
func NewNamespaceBackupRunner(
	backupExecutor backupexecutor.Backup,
	backendService BackupWriter,
	pathService PathService,
) NamespaceBackupRunner {
	return &NamespaceBackupRunnerImpl{
		backupExecutor: backupExecutor,
		backendService: backendService,
		pathService:    pathService,
	}
}

type NamespaceBackupRunnerImpl struct {
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
func (e *NamespaceBackupRunnerImpl) Run(
	ctx context.Context,
	routine *model.BackupRoutine,
	namespace string,
	runSpec model.BackupRunSpec,
	logger *slog.Logger,
) CancelableBackupHandler {
	backupFolder := e.pathService.GetBackupPath(routine.Name, runSpec.Type, namespace, runSpec.StartTime)

	return newRetryableBackupHandler(
		ctx,
		*routine.BackupPolicy.GetRetryPolicyOrDefault(),
		retryableBackupCallbacks{
			Start: func(ctx context.Context) (backupexecutor.BackupHandler, error) {
				return e.backupExecutor.Run(ctx, routine, runSpec.TimeBounds, namespace, backupFolder, logger)
			},
			OnFail: func(ctx context.Context) {
				path := e.pathService.GetTimestampPath(routine.Name, runSpec.StartTime, runSpec.Type)
				e.deleteFolder(ctx, routine, path, logger)
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
					routine.BackupPolicy,
				)

				return e.writeBackupMetadata(ctx, routine, metadata, backupFolder, logger)
			},
			OnRetry: func() {
				prometheus.ObserveBackupEvent(routine.Name, runSpec.Type, prometheus.OutcomeRetry, 0, runSpec.StartTime)
			},
		},
		logger,
	)
}

// deleteFolder removes backup data under the given path on failure or cancel; logs
// errors except when the context was canceled during delete.
func (e *NamespaceBackupRunnerImpl) deleteFolder(
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
func (e *NamespaceBackupRunnerImpl) writeBackupMetadata(
	ctx context.Context,
	routine *model.BackupRoutine,
	metadata model.BackupMetadata,
	backupFolder string,
	logger *slog.Logger,
) error {
	if err := e.backendService.WriteBackupMetadata(ctx, routine, backupFolder, metadata); err != nil {
		return fmt.Errorf("failed to write backup metadata to %q: %w", backupFolder, err)
	}

	logger.Info("Wrote backup metadata",
		slog.Any("folder", backupFolder),
		slog.Any("metadata", metadata))

	return nil
}
