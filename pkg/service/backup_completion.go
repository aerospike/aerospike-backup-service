package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// BackupCompletionHandler updates the registry when a backup run ends. After a full backup it
// also starts retention cleanup, and a cluster configuration backup if the routine asks for it.
type BackupCompletionHandler interface {
	// OnSuccess is called after a backup completes successfully. It returns immediately:
	// the registry update, and for full backups the retention cleanup and cluster
	// configuration backup, continue in the background.
	OnSuccess(
		ctx context.Context,
		routine *model.BackupRoutine,
		backupType model.BackupType,
		timestamp time.Time,
		logger *slog.Logger,
	)
	// OnFailure is called when a backup run has failed and every namespace of the run has
	// finished. It clears the failed state in the registry and removes the run's folder.
	OnFailure(
		ctx context.Context,
		routine *model.BackupRoutine,
		backupType model.BackupType,
		timestamp time.Time,
		logger *slog.Logger,
	)
}

type backupCompletionHandler struct {
	registry            BackupStateRegistry
	retentionManager    BackupRetentionManager
	clusterConfigWriter ClusterConfigWriter
	backupWriter        BackupWriter
	pathService         PathService
}

// NewBackupCompletionHandler returns a BackupCompletionHandler.
func NewBackupCompletionHandler(
	registry BackupStateRegistry,
	retentionManager BackupRetentionManager,
	clusterConfigWriter ClusterConfigWriter,
	backupWriter BackupWriter,
	pathService PathService,
) BackupCompletionHandler {
	return &backupCompletionHandler{
		registry:            registry,
		retentionManager:    retentionManager,
		clusterConfigWriter: clusterConfigWriter,
		backupWriter:        backupWriter,
		pathService:         pathService,
	}
}

var _ BackupCompletionHandler = (*backupCompletionHandler)(nil)

func (h *backupCompletionHandler) OnSuccess(
	ctx context.Context,
	routine *model.BackupRoutine,
	backupType model.BackupType,
	timestamp time.Time,
	logger *slog.Logger,
) {
	go h.registry.BackupSucceeded(ctx, routine, backupType)

	if backupType != model.BackupTypeFull {
		return
	}

	go func() {
		if err := h.retentionManager.ApplyRetention(ctx, routine); err != nil {
			if errors.Is(err, context.Canceled) {
				logger.Info("Old backups cleanup context canceled")
				return
			}
			logger.Error("Failed to clean up old backups", attr.Error(err))
		}
	}()

	go func() {
		if routine.BackupPolicy.WithClusterConfigOrDefault() {
			if err := h.clusterConfigWriter.Write(ctx, routine, timestamp); err != nil {
				if errors.Is(err, context.Canceled) {
					logger.Info("Cluster configuration backup context canceled")
					return
				}

				logger.Error("Failed to backup cluster configuration", attr.Error(err))
			}
		}
	}()
}

func (h *backupCompletionHandler) OnFailure(
	ctx context.Context,
	routine *model.BackupRoutine,
	backupType model.BackupType,
	timestamp time.Time,
	logger *slog.Logger,
) {
	h.registry.BackupFailed(routine.Name, backupType)

	// Every namespace of the run has finished, so nothing writes here any more. The folder still
	// holds the data and metadata of the namespaces that did succeed, and the cluster
	// configuration if one was taken: a run that failed must leave no restorable backup behind.
	// The cleanup outlives a canceled run, which is how a canceled backup gets cleaned up too.
	runFolder := h.pathService.GetTimestampPath(routine.Name, timestamp, backupType)
	if err := h.backupWriter.Delete(context.WithoutCancel(ctx), routine, runFolder); err != nil {
		logger.Error("Failed to remove the folder of a failed backup",
			slog.String("folder", runFolder), attr.Error(err))
	}
}
