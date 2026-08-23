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
	// OnFailure is called when a backup fails (clears failed state in registry).
	OnFailure(routine *model.BackupRoutine, backupType model.BackupType)
}

type backupCompletionHandler struct {
	registry            backupRunCoordinator
	retentionManager    BackupRetentionManager
	clusterConfigWriter ClusterConfigWriter
}

// NewBackupCompletionHandler returns a BackupCompletionHandler.
func NewBackupCompletionHandler(
	registry backupRunCoordinator,
	retentionManager BackupRetentionManager,
	clusterConfigWriter ClusterConfigWriter,
) BackupCompletionHandler {
	return &backupCompletionHandler{
		registry:            registry,
		retentionManager:    retentionManager,
		clusterConfigWriter: clusterConfigWriter,
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
	go h.registry.recordSuccessfulBackup(routine, backupType)

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
		if routine.BackupPolicy.WithClusterConfig != nil && *routine.BackupPolicy.WithClusterConfig {
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
	routine *model.BackupRoutine,
	backupType model.BackupType,
) {
	h.registry.clearFailedBackup(routine.Name, backupType)
}
