package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/prometheus"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/syncutil"
	"github.com/aerospike/backup-go/models"
)

// NamespaceBackupRunner runs the backup pipeline for one namespace: retries, metadata writes on
// success, and removal of the folders of attempts that a later attempt replaced.
type NamespaceBackupRunner interface {
	// Run starts backup execution for a single namespace and returns a cancelable handler
	// once the backup pipeline is running. It fails when the pipeline cannot be started.
	// scanLimiter is a per-routine limiter shared across all namespace backups within
	// a single routine run to ensure fair resource allocation.
	Run(
		ctx context.Context,
		routine *model.BackupRoutine,
		namespace string,
		runSpec model.BackupRunSpec,
		scanLimiter syncutil.Limiter,
		logger *slog.Logger,
	) (CancelableBackupHandler, error)
}

// NewNamespaceBackupRunner returns a NamespaceBackupRunner.
func NewNamespaceBackupRunner(
	backupExecutor backupexecutor.Backup,
	backupWriter BackupWriter,
	pathService PathService,
) NamespaceBackupRunner {
	return &namespaceBackupRunner{
		backupExecutor: backupExecutor,
		backupWriter:   backupWriter,
		pathService:    pathService,
	}
}

type namespaceBackupRunner struct {
	backupExecutor backupexecutor.Backup
	backupWriter   BackupWriter
	pathService    PathService
}

var _ NamespaceBackupRunner = (*namespaceBackupRunner)(nil)

// CancelableBackupHandler is a [backupexecutor.BackupHandler] that can be canceled
// while a backup is in progress.
type CancelableBackupHandler interface {
	backupexecutor.BackupHandler
	// Cancel requests cancellation of the in-flight backup.
	Cancel()
}

// Run starts a retryable backup for one namespace via [backupexecutor.Backup.Run],
// writing each attempt to a folder of its own in the routine's storage layout, and returns
// once the pipeline is running. A run that gives up before starting one returns its error.
// scanLimiter is a per-routine limiter shared across all namespace backups within
// a single routine run to ensure fair resource allocation.
func (e *namespaceBackupRunner) Run(
	ctx context.Context,
	routine *model.BackupRoutine,
	namespace string,
	runSpec model.BackupRunSpec,
	scanLimiter syncutil.Limiter,
	logger *slog.Logger,
) (CancelableBackupHandler, error) {
	// Every attempt writes to a folder of its own under the run's timestamp, so a retry never
	// shares a folder with the attempt it replaces, and the run keeps one timestamp however many
	// attempts its namespaces needed. A failed attempt's folder has no metadata, which keeps it out
	// of the catalog; it is removed once a later attempt has completed.
	attempt := 1
	attemptFolder := func() string {
		return e.pathService.GetBackupAttemptPath(routine.Name, runSpec.Type, namespace, runSpec.StartTime, attempt)
	}
	var failedFolders []string

	h, err := startRetryableBackup(
		ctx,
		*routine.BackupPolicy.GetRetryPolicyOrDefault(),
		retryableBackupCallbacks{
			Start: func(ctx context.Context) (backupexecutor.BackupHandler, error) {
				return e.backupExecutor.Run(ctx, routine, runSpec.TimeBounds, namespace, attemptFolder(), scanLimiter, logger)
			},
			OnSuccess: func(ctx context.Context, stats *models.BackupStats) error {
				if err := e.completeAttempt(ctx, routine, namespace, runSpec, stats, attemptFolder(), logger); err != nil {
					return err
				}

				e.removeFailedAttempts(ctx, routine, failedFolders, logger)

				return nil
			},
			OnRetry: func() {
				prometheus.ObserveBackupEvent(routine.Name, runSpec.Type, prometheus.OutcomeRetry, 0)
				failedFolders = append(failedFolders, attemptFolder())
				attempt++
			},
		},
		logger,
	)
	if err != nil {
		return nil, err
	}

	return h, nil
}

// removeFailedAttempts removes the folders of attempts that a later attempt has replaced. The
// namespace is already backed up, so a failure here is only logged: storage may not allow
// deletes, and the folders have no metadata, so they stay out of the catalog until retention
// removes the run.
func (e *namespaceBackupRunner) removeFailedAttempts(
	ctx context.Context,
	routine *model.BackupRoutine,
	folders []string,
	logger *slog.Logger,
) {
	for _, folder := range folders {
		if err := e.backupWriter.Delete(ctx, routine, folder); err != nil {
			logger.Warn("Failed to remove the folder of a failed backup attempt",
				slog.String("folder", folder), attr.Error(err))
		}
	}
}

// completeAttempt writes the metadata that makes an attempt's folder a backup. An empty
// incremental backup has nothing to restore, so its folder gets none.
func (e *namespaceBackupRunner) completeAttempt(
	ctx context.Context,
	routine *model.BackupRoutine,
	namespace string,
	runSpec model.BackupRunSpec,
	stats *models.BackupStats,
	backupFolder string,
	logger *slog.Logger,
) error {
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
}

// writeBackupMetadata persists backup metadata to storage and logs the folder on success.
func (e *namespaceBackupRunner) writeBackupMetadata(
	ctx context.Context,
	routine *model.BackupRoutine,
	metadata model.BackupMetadata,
	backupFolder string,
	logger *slog.Logger,
) error {
	if err := e.backupWriter.WriteBackupMetadata(ctx, routine, backupFolder, metadata); err != nil {
		return fmt.Errorf("failed to write backup metadata to %q: %w", backupFolder, err)
	}

	logger.Info("Wrote backup metadata",
		slog.Any("folder", backupFolder),
		slog.Any("metadata", metadata))

	return nil
}
