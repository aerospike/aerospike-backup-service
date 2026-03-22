package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
)

// BackupCompletionHandler runs all actions after a backup succeeds or fails:
// record/clear registry state, retention, and cluster config (on success for full backup).
type BackupCompletionHandler interface {
	// OnSuccess is called after a backup completes successfully.
	// For full backups it also runs retention and backs up cluster configuration.
	OnSuccess(
		ctx context.Context,
		routine *model.BackupRoutine,
		jobType model.BackupType,
		timestamp time.Time,
		logger *slog.Logger,
	)
	// OnFailure is called when a backup fails (clears failed state in registry).
	OnFailure(routine *model.BackupRoutine, jobType model.BackupType)
}

type backupCompletionHandler struct {
	registry            RunningBackupsRegistry
	retentionManager    RetentionManager
	clusterConfigWriter ClusterConfigWriter
}

// NewBackupCompletionHandler returns a BackupCompletionHandler that delegates to
// registry, retentionManager, and clusterConfigWriter.
func NewBackupCompletionHandler(
	registry RunningBackupsRegistry,
	retentionManager RetentionManager,
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
	jobType model.BackupType,
	timestamp time.Time,
	logger *slog.Logger,
) {
	go h.registry.recordSuccessfulBackup(routine.Name, jobType, timestamp)

	if jobType != model.BackupJobTypeFull {
		return
	}

	go func() {
		if err := h.retentionManager.deleteOldBackups(ctx, routine); err != nil {
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
	jobType model.BackupType,
) {
	h.registry.clearFailedBackup(routine.Name, jobType)
}

// reportBackupOutcome classifies the backup terminal outcome from err and logs it.
// It also emits the corresponding backup event prometheus metrics.
func reportBackupOutcome(
	routineName string,
	backupType model.BackupType,
	duration time.Duration,
	err error,
	logger *slog.Logger,
) {
	operation := backupType.String()

	if err == nil {
		logger.Debug(operation+" finished", slog.Duration("duration", duration))
		observeBackupEvent(routineName, backupType, BackupOutcomeSuccess, duration)
		return
	}

	if errors.Is(err, errBackupSkipped) {
		logger.Debug(operation + " skipped")
		observeBackupEvent(routineName, backupType, BackupOutcomeSkip, 0)
		return
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		logger.Info(operation + " context canceled")
		observeBackupEvent(routineName, backupType, BackupOutcomeCanceled, duration)
		return
	}

	var aerr *as.AerospikeError
	if errors.As(err, &aerr) {
		logger.Error(
			operation+" failed due to Aerospike error",
			slog.Int("resultCode", int(aerr.ResultCode)),
			attr.Error(err),
		)
	} else {
		logger.Error(operation+" failed", attr.Error(err))
	}

	observeBackupEvent(routineName, backupType, BackupOutcomeFailure, duration)
}
