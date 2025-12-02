package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/timeutil"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/pkg/asinfo"
)

var (
	errBackupSkipped   = errors.New("full backup skipped")
	nonRetryableErrors = []error{asinfo.ErrNoNode}
)

// BackupRoutineOrchestrator orchestrates the execution of a single backup routine (both full and incremental).
// It manages all necessary preparations, executes the backup process, handles post-processing, and updates metrics.
type BackupRoutineOrchestrator struct {
	logger              *slog.Logger
	runner              *BackupNamespaceRunner
	routine             *model.BackupRoutine
	clusterConfigWriter ClusterConfigWriter
	clientManager       aerospike.ClientManager
	registry            RunningBackupsRegistry
	retentionManager    RetentionManager

	fullBackupLock sync.Mutex
}

var _ backupRunner = (*BackupRoutineOrchestrator)(nil)

// BackupComponents holds all components required to execute a backup routine.
type BackupComponents struct {
	clientManager    aerospike.ClientManager // Retrieves the Aerospike client before starting the backup.
	backupExecutor   backupexecutor.Backup   // Executes the backup using the backup-go library.
	registry         RunningBackupsRegistry  // Stores the backup handler during execution.
	retentionManager RetentionManager        // Deletes old backups after a successful backup.
	backendService   BackupWriter            // Writes backup metadata after a successful backup and
	// deletes created files if the backup fails.
	clusterConfigWriter ClusterConfigWriter // Backs up cluster configuration.
}

func NewBackupComponents(
	clientManager aerospike.ClientManager,
	backupExecutor backupexecutor.Backup,
	registry RunningBackupsRegistry,
	retentionManager RetentionManager,
	backendService BackupWriter,
	clusterConfigWriter ClusterConfigWriter,
) *BackupComponents {
	return &BackupComponents{
		clientManager:       clientManager,
		backupExecutor:      backupExecutor,
		registry:            registry,
		retentionManager:    retentionManager,
		backendService:      backendService,
		clusterConfigWriter: clusterConfigWriter,
	}
}

func newOrchestrator(
	routine *model.BackupRoutine,
	h *BackupComponents,
	pathService PathService,
) *BackupRoutineOrchestrator {
	logger := slog.With(attr.Routine(routine.Name))
	retry := newRetryExecutor(*routine.BackupPolicy.GetRetryPolicyOrDefault(), logger, nonRetryableErrors...)
	runner := NewBackupNamespaceRunner(routine, h.backupExecutor, retry, h.backendService, logger, pathService)
	return &BackupRoutineOrchestrator{
		routine:             routine,
		runner:              runner,
		clusterConfigWriter: h.clusterConfigWriter,
		clientManager:       h.clientManager,
		registry:            h.registry,
		retentionManager:    h.retentionManager,
		logger:              logger,
	}
}

func (h *BackupRoutineOrchestrator) runFullBackup(ctx context.Context, now time.Time) {
	duration, err := timeutil.MeasureDuration(func() error {
		return h.runFullBackupInternal(ctx, now)
	})

	if err != nil {
		if errors.Is(err, errBackupSkipped) {
			h.logger.Debug("Full backup skipped")
			observeBackupEvent(h.routine.Name, jobTypeFull, BackupOutcomeSkip, 0)
		} else {
			h.logger.Error("Full backup failed", attr.Error(err))
			observeBackupEvent(h.routine.Name, jobTypeFull, BackupOutcomeFailure, duration)
		}
	} else {
		h.logger.Debug("Full backup finished")
		observeBackupEvent(h.routine.Name, jobTypeFull, BackupOutcomeSuccess, duration)
	}
}

func (h *BackupRoutineOrchestrator) runFullBackupInternal(ctx context.Context, now time.Time) error {
	h.fullBackupLock.Lock()
	if h.skipFullBackup() {
		h.fullBackupLock.Unlock()
		return errBackupSkipped
	}

	client, namespaces, err := h.prepareCluster(ctx)
	if err != nil {
		h.fullBackupLock.Unlock()
		return err
	}
	defer h.clientManager.Close(client)

	timeBounds := h.createTimeBounds(jobTypeFull, now)
	backupHandler := startNamespacesBackup(ctx, h.runner, client, namespaces, timeBounds, now, h.routine, jobTypeFull)

	h.registry.register(h.routine.Name, jobTypeFull, backupHandler)
	// The lock must be held until the backup is registered.
	h.fullBackupLock.Unlock()

	h.backupClusterConfiguration(ctx, now)

	if err = backupHandler.Wait(ctx); err != nil {
		h.registry.remove(h.routine.Name, jobTypeFull)
		return fmt.Errorf("backup failed: %w", err)
	}

	go h.registry.unregister(h.routine.Name, jobTypeFull, now)
	go h.deleteOldBackups(ctx, h.routine)

	return nil
}

func (h *BackupRoutineOrchestrator) backupClusterConfiguration(ctx context.Context, now time.Time) {
	// backup configuration only if WithClusterConfig is explicitly set to true.
	if h.routine.BackupPolicy.WithClusterConfig == nil || !*h.routine.BackupPolicy.WithClusterConfig {
		return
	}

	if err := h.clusterConfigWriter.Write(ctx, h.routine, now); err != nil {
		h.logger.Warn("Failed to backup cluster configuration", attr.Error(err))
	}
}

func (h *BackupRoutineOrchestrator) skipFullBackup() bool {
	currentStat := h.registry.GetRoutineState(h.routine)
	if currentStat.Full != nil {
		h.logger.Info("Full backup is currently in progress, skipping another full backup")
		return true
	}

	return false
}

func (h *BackupRoutineOrchestrator) deleteOldBackups(ctx context.Context, routine *model.BackupRoutine) {
	err := h.retentionManager.deleteOldBackups(ctx, routine)
	if err != nil {
		h.logger.Error("failed to clean up old backups", attr.Error(err))
	}
}

func (h *BackupRoutineOrchestrator) prepareCluster(ctx context.Context) (aerospike.Client, []string, error) {
	client, err := h.clientManager.GetClient(ctx, h.routine.SourceCluster)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get backup client: %w", err)
	}

	namespaces, err := h.resolveNamespaces(ctx, h.routine.Namespaces, client.InfoClient())
	if err != nil {
		h.clientManager.Close(client)
		return nil, nil, fmt.Errorf("cannot retrieve namespaces from source cluster: %w", err)
	}

	return client, namespaces, nil
}

// resolveNamespaces returns the list of namespaces to back up.
// If `namespaces` is empty, it fetches all namespaces from the cluster via the provided client.
func (h *BackupRoutineOrchestrator) resolveNamespaces(
	ctx context.Context,
	namespaces []string,
	ig backup.InfoGetter,
) ([]string, error) {
	if len(namespaces) == 0 {
		return ig.GetNamespacesList(ctx)
	}

	return namespaces, nil
}

func (h *BackupRoutineOrchestrator) createTimeBounds(jobType jobType, now time.Time) model.TimeBounds {
	var (
		fromTime *time.Time
		toTime   *time.Time
	)

	if jobType == jobTypeIncremental {
		fromTime = h.registry.GetRoutineState(h.routine).LastRunTime.LatestRun()
	}

	if h.routine.BackupPolicy.IsSealedOrDefault() {
		toTime = &now
	}

	return model.TimeBounds{FromTime: fromTime, ToTime: toTime}
}

func (h *BackupRoutineOrchestrator) runIncrementalBackup(ctx context.Context, now time.Time) {
	if h.skipIncrementalBackup(now) {
		h.logger.Debug("Incremental backup skipped")
		observeBackupEvent(h.routine.Name, jobTypeIncremental, BackupOutcomeSkip, 0)
		return
	}

	duration, err := timeutil.MeasureDuration(func() error {
		return h.runIncrementalBackupInternal(ctx, now)
	})
	if err != nil {
		observeBackupEvent(h.routine.Name, jobTypeIncremental, BackupOutcomeFailure, duration)
		h.logger.Error("Incremental backup failed", attr.Error(err))
	} else {
		observeBackupEvent(h.routine.Name, jobTypeIncremental, BackupOutcomeSuccess, duration)
		h.logger.Debug("Incremental backup finished")
	}
}

func (h *BackupRoutineOrchestrator) skipIncrementalBackup(now time.Time) bool {
	currentStat := h.registry.GetRoutineState(h.routine)

	// Skip if no initial full backup has been completed
	if currentStat.LastRunTime.NoFullBackup() {
		h.logger.Debug("Skipping incremental backup: initial full backup not yet completed")
		return true
	}

	// Concurrent incremental are only allowed when explicitly set (default is false).
	if allowConcurrent := h.routine.BackupPolicy.ConcurrentIncremental != nil &&
		*h.routine.BackupPolicy.ConcurrentIncremental; allowConcurrent {
		return false
	}

	switch {
	case currentStat.Full != nil:
		h.logger.Debug("Skipping incremental backup: full backup in progress")
		return true
	case currentStat.Incremental != nil:
		h.logger.Debug("Skipping incremental backup: another incremental backup in progress")
		return true
	case timeutil.IsCronFireTime(h.routine.IntervalCron, now):
		h.logger.Debug("Skipping incremental backup: full backup scheduled at same time")
		return true
	}

	return false
}

func (h *BackupRoutineOrchestrator) runIncrementalBackupInternal(ctx context.Context, now time.Time) error {
	client, namespaces, err := h.prepareCluster(ctx)
	if err != nil {
		return err
	}

	defer h.clientManager.Close(client)

	timeBounds := h.createTimeBounds(jobTypeIncremental, now)
	backupHandler := startNamespacesBackup(ctx,
		h.runner, client, namespaces, timeBounds, now, h.routine, jobTypeIncremental)
	h.registry.register(h.routine.Name, jobTypeIncremental, backupHandler)

	if err := backupHandler.Wait(ctx); err != nil {
		h.registry.remove(h.routine.Name, jobTypeIncremental)
		return err
	}

	go h.registry.unregister(h.routine.Name, jobTypeIncremental, now)

	return nil
}
