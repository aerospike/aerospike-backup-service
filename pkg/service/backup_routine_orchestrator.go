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
)

var errBackupSkipped = errors.New("full backup skipped")

// BackupRoutineOrchestrator orchestrates the execution of a single backup routine (both full and incremental).
// It manages all necessary preparations, executes the backup process, handles post-processing, and updates metrics.
type BackupRoutineOrchestrator struct {
	retry               executor
	logger              *slog.Logger
	runner              *BackupNamespaceRunner
	routineName         string
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

func newOrchestrator(routineName string, config *model.Config, h *BackupComponents) *BackupRoutineOrchestrator {
	routine, _ := config.Routine(routineName)
	logger := slog.With(attr.Routine(routineName))
	retry := newRetryExecutor(*routine.BackupPolicy.GetRetryPolicyOrDefault(), logger)
	return &BackupRoutineOrchestrator{
		routineName:         routineName,
		routine:             routine,
		runner:              NewBackupNamespaceRunner(routineName, h.backupExecutor, retry, h.backendService, logger),
		retry:               retry,
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
			observeBackupEvent(h.routineName, jobTypeFull, BackupOutcomeSkip, 0)
		} else {
			h.logger.Error("Full backup failed", attr.Error(err))
			observeBackupEvent(h.routineName, jobTypeFull, BackupOutcomeFailure, duration)
		}
	} else {
		h.logger.Debug("Full backup finished")
		observeBackupEvent(h.routineName, jobTypeFull, BackupOutcomeSuccess, duration)
	}
}

func (h *BackupRoutineOrchestrator) runFullBackupInternal(ctx context.Context, now time.Time) error {
	h.fullBackupLock.Lock()
	if h.skipFullBackup() {
		h.fullBackupLock.Unlock()
		return errBackupSkipped
	}

	client, namespaces, err := h.prepareCluster(h.retry)
	if err != nil {
		h.fullBackupLock.Unlock()
		return err
	}
	defer h.clientManager.Close(client)

	timeBounds := h.createTimeBounds(jobTypeFull, now)
	backupHandler := startNamespacesBackup(ctx, h.runner, client, namespaces, timeBounds, now, h.routine, jobTypeFull)

	h.registry.register(h.routineName, jobTypeFull, backupHandler)
	// The lock must be held until the backup is registered.
	h.fullBackupLock.Unlock()

	h.backupClusterConfiguration(ctx, now)

	if err = backupHandler.Wait(ctx); err != nil {
		h.registry.remove(h.routineName, jobTypeFull)
		return fmt.Errorf("backup failed: %w", err)
	}

	go h.registry.unregister(h.routineName, jobTypeFull, now)
	go h.deleteOldBackups(ctx, h.routineName)

	return nil
}

func (h *BackupRoutineOrchestrator) backupClusterConfiguration(ctx context.Context, now time.Time) {
	// backup configuration only if WithClusterConfig is explicitly set to true.
	if h.routine.BackupPolicy.WithClusterConfig == nil || !*h.routine.BackupPolicy.WithClusterConfig {
		return
	}

	if err := h.clusterConfigWriter.Write(ctx, h.routineName, now); err != nil {
		h.logger.Warn("Failed to backup cluster configuration", attr.Error(err))
	}
}

func (h *BackupRoutineOrchestrator) skipFullBackup() bool {
	currentStat := h.registry.GetRoutineState(h.routineName)
	if currentStat.Full != nil {
		h.logger.Info("Full backup is currently in progress, skipping another full backup")
		return true
	}

	return false
}

func (h *BackupRoutineOrchestrator) deleteOldBackups(ctx context.Context, routineName string) {
	err := h.retentionManager.deleteOldBackups(ctx, routineName)
	if err != nil {
		h.logger.Error("failed to clean up old backups", attr.Error(err))
	}
}

func (h *BackupRoutineOrchestrator) prepareCluster(retry executor) (aerospike.Client, []string, error) {
	var (
		client     aerospike.Client
		namespaces []string
	)

	err := retry.run("cluster connection", func() error {
		var err error
		client, err = h.clientManager.GetClient(h.routine.SourceCluster)
		if err != nil {
			return fmt.Errorf("cannot get backup client: %w", err)
		}
		namespaces, err = h.resolveNamespaces(h.routine.Namespaces, client.InfoClient())
		if err != nil {
			h.clientManager.Close(client)
			return fmt.Errorf("cannot retrieve namespaces from source cluster: %w", err)
		}

		return nil
	}, func() {})

	return client, namespaces, err
}

// resolveNamespaces returns the list of namespaces to back up.
// If `namespaces` is empty, it fetches all namespaces from the cluster via the provided client.
func (h *BackupRoutineOrchestrator) resolveNamespaces(namespaces []string, ig backup.InfoGetter) ([]string, error) {
	if len(namespaces) == 0 {
		return ig.GetNamespacesList()
	}

	return namespaces, nil
}

func (h *BackupRoutineOrchestrator) createTimeBounds(jobType jobType, now time.Time) model.TimeBounds {
	var (
		fromTime *time.Time
		toTime   *time.Time
	)

	if jobType == jobTypeIncremental {
		fromTime = h.registry.GetRoutineState(h.routineName).LastRunTime.LatestRun()
	}

	if h.routine.BackupPolicy.IsSealedOrDefault() {
		toTime = &now
	}

	return model.TimeBounds{FromTime: fromTime, ToTime: toTime}
}

func (h *BackupRoutineOrchestrator) runIncrementalBackup(ctx context.Context, now time.Time) {
	if h.skipIncrementalBackup(now) {
		h.logger.Debug("Incremental backup skipped")
		observeBackupEvent(h.routineName, jobTypeIncremental, BackupOutcomeSkip, 0)
		return
	}

	duration, err := timeutil.MeasureDuration(func() error {
		return h.runIncrementalBackupInternal(ctx, now)
	})
	if err != nil {
		observeBackupEvent(h.routineName, jobTypeIncremental, BackupOutcomeFailure, duration)
		h.logger.Error("Incremental backup failed", attr.Error(err))
	} else {
		observeBackupEvent(h.routineName, jobTypeIncremental, BackupOutcomeSuccess, duration)
		h.logger.Debug("Incremental backup finished")
	}
}

func (h *BackupRoutineOrchestrator) skipIncrementalBackup(now time.Time) bool {
	currentStat := h.registry.GetRoutineState(h.routineName)

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
	client, namespaces, err := h.prepareCluster(&simpleExecutor{})
	if err != nil {
		return err
	}

	defer h.clientManager.Close(client)

	timeBounds := h.createTimeBounds(jobTypeIncremental, now)
	backupHandler := startNamespacesBackup(ctx,
		h.runner, client, namespaces, timeBounds, now, h.routine, jobTypeIncremental)
	h.registry.register(h.routineName, jobTypeIncremental, backupHandler)

	if err := backupHandler.Wait(ctx); err != nil {
		h.registry.remove(h.routineName, jobTypeIncremental)
		return err
	}

	go h.registry.unregister(h.routineName, jobTypeIncremental, now)

	return nil
}
