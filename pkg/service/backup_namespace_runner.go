package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go/models"
)

// NamespaceBackupExecutor runs a backup for one namespace.
type NamespaceBackupExecutor struct {
	clientManager  aerospike.ClientManager
	backupExecutor backupexecutor.Backup
	backendService BackupWriter
	pathService    PathService
}

func NewNamespaceBackupExecutor(
	clientManager aerospike.ClientManager,
	backupExecutor backupexecutor.Backup,
	backendService BackupWriter,
	pathService PathService,
) *NamespaceBackupExecutor {
	return &NamespaceBackupExecutor{
		clientManager:  clientManager,
		backupExecutor: backupExecutor,
		backendService: backendService,
		pathService:    pathService,
	}
}

// StartBackup resolves namespaces for the routine and starts one backup handler per namespace.
func (e *NamespaceBackupExecutor) StartBackup(
	ctx context.Context,
	logger *slog.Logger,
	backupRoutine *model.BackupRoutine,
	runSpec model.BackupRunSpec,
) (*BackupNamespacesOperation, error) {
	namespaces := backupRoutine.Namespaces
	if len(namespaces) == 0 {
		client, err := e.clientManager.GetClient(ctx, backupRoutine.SourceCluster, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to get backup client: %w", err)
		}
		namespaces, err = client.InfoClient().GetNamespacesList(ctx)
		e.clientManager.Close(client)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve namespaces from source cluster: %w", err)
		}
	}

	op := &BackupNamespacesOperation{
		handlers: make(map[string]CancelableBackupHandler, len(namespaces)),
	}

	for _, namespace := range namespaces {
		op.handlers[namespace] = e.Run(ctx, logger, backupRoutine, namespace, runSpec)
	}

	return op, nil
}

// CancelableBackupHandler extends BackupHandler with support for canceling the backup.
type CancelableBackupHandler interface {
	backupexecutor.BackupHandler
	// Cancel cancels the backup operation.
	Cancel()
}

// Run executes the backup operation for the namespace.
func (e *NamespaceBackupExecutor) Run(
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
				e.deleteFolder(
					ctx,
					logger,
					backupRoutine,
					e.pathService.GetTimestampPath(backupRoutine.Name, runSpec.StartTime, runSpec.Type),
				)
			},
			OnSuccess: func(ctx context.Context, stats *models.BackupStats) error {
				if runSpec.Type == model.BackupJobTypeIncremental && stats.IsEmpty() {
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

func (e *NamespaceBackupExecutor) deleteFolder(ctx context.Context, logger *slog.Logger, routine *model.BackupRoutine, path string) {
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

func (e *NamespaceBackupExecutor) writeBackupMetadata(
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
