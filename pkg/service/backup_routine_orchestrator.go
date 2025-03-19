package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/aerospike/backup-go"
)

// BackupRoutineOrchestrator orchestrates the execution of a single backup routine (both full and incremental).
// It manages all necessary preparations, executes the backup process, handles post-processing, and updates metrics.
type BackupRoutineOrchestrator struct {
	backupService       backupexecutor.Backup
	backupRoutine       *model.BackupRoutine
	namespaces          []string
	retry               executor
	clientManager       aerospike.ClientManager
	logger              *slog.Logger
	clusterConfigWriter ClusterConfigWriter
	retentionManager    RetentionManager
	runner              *BackupNamespaceRunner
	routineName         string
	registry            RunningBackupsRegistry
}

var _ backupRunner = (*BackupRoutineOrchestrator)(nil)

// ClusterConfigWriter handles writing cluster configuration to storage.
type ClusterConfigWriter interface {
	Write(ctx context.Context, client aerospike.Cluster, timestamp time.Time)
}

// BackupHandlerHolder stores backupRunners by routine name.
type BackupHandlerHolder = *util.SafeMap[string, backupRunner]

func NewBackupHandlerHolder() BackupHandlerHolder {
	return util.NewSafeMap[string, backupRunner]()
}

// newBackupRoutineOrchestrator returns a new BackupRoutineOrchestrator instance.
func newBackupRoutineOrchestrator(
	clientManager aerospike.ClientManager,
	backupService backupexecutor.Backup,
	routineName string,
	routine *model.BackupRoutine,
	backupBackend BackupMetadataReaderWriter,
	registry RunningBackupsRegistry,
	retentionManager RetentionManager,
) *BackupRoutineOrchestrator {
	backupPolicy := routine.BackupPolicy
	backupStorage := routine.Storage
	logger := slog.Default().With(slog.String("routine", routineName))
	retry := newRetryExecutor(
		backupPolicy.GetRetryPolicyOrDefault(),
		logger)
	return &BackupRoutineOrchestrator{
		runner: NewBackupNamespaceRunner(
			routineName,
			backupService,
			retry,
			backupBackend,
			logger,
		),

		backupService: backupService,
		backupRoutine: routine,
		routineName:   routineName,
		namespaces:    routine.Namespaces,
		clientManager: clientManager,
		clusterConfigWriter: NewClusterConfigWriter(
			backupStorage,
			routineName,
			backupPolicy,
			logger),
		logger:           logger,
		retry:            retry,
		registry:         registry,
		retentionManager: retentionManager,
	}
}

func (h *BackupRoutineOrchestrator) runFullBackup(ctx context.Context, now time.Time) {
	duration, err := util.MeasureDuration(func() error {
		return h.runFullBackupInternal(ctx, now)
	})

	if err != nil {
		h.logger.Error("Full backup failed", slog.Any("error", err))
		backupFailureCounter.Inc()
	} else {
		h.logger.Debug("Finished full backup", slog.Int64("time", now.UnixMilli()))
		backupDurationGauge.Set(float64(duration.Milliseconds()))
		backupCounter.Inc()
	}
}

func (h *BackupRoutineOrchestrator) runFullBackupInternal(ctx context.Context, now time.Time) error {
	if h.skipFullBackup() {
		backupSkippedCounter.Inc()
		return nil
	}

	client, namespaces, err := h.prepareCluster(h.retry)
	if err != nil {
		return err
	}
	defer h.clientManager.Close(client)

	h.clusterConfigWriter.Write(ctx, client.AerospikeClient(), now)

	timeBounds := h.createTimeBounds(jobTypeFull, now)
	backupHandler := startNamespacesBackup(ctx,
		h.runner, client, namespaces, timeBounds, now, h.backupRoutine, jobTypeFull)

	h.registry.register(h.routineName, jobTypeFull, backupHandler)

	if err = backupHandler.Wait(ctx); err != nil {
		h.registry.remove(h.routineName, jobTypeFull)
		return fmt.Errorf("backup failed: %w", err)
	}
	go h.registry.unregister(h.routineName, jobTypeFull, now)

	go h.deleteOldBackups(ctx)

	return nil
}

func (h *BackupRoutineOrchestrator) skipFullBackup() bool {
	currentStat := h.registry.GetRoutineState(h.routineName)
	if currentStat.Full != nil {
		// This can happen in rare scenario, when user re-applied config
		// while backup is running and started same routine backup.
		h.logger.Debug("Full backup is currently in progress, skipping another full backup")
		return true
	}

	return false
}

func (h *BackupRoutineOrchestrator) deleteOldBackups(ctx context.Context) {
	err := h.retentionManager.deleteOldBackups(ctx, h.routineName)
	if err != nil {
		h.logger.Error("failed to clean up old backups", slog.Any("error", err))
	}
}

func (h *BackupRoutineOrchestrator) prepareCluster(retry executor) (*backup.Client, []string, error) {
	var (
		client     *backup.Client
		namespaces []string
	)

	err := retry.run("cluster connection", func() error {
		var err error
		client, err = h.clientManager.GetClient(h.backupRoutine.SourceCluster)
		if err != nil {
			return fmt.Errorf("cannot get backup client: %w", err)
		}
		namespaces, err = aerospike.ResolveNamespaces(h.namespaces, client.AerospikeClient())
		if err != nil {
			return fmt.Errorf("cannot retrieve namespaces from source cluster: %w", err)
		}

		return nil
	})

	return client, namespaces, err
}

func (h *BackupRoutineOrchestrator) createTimeBounds(jobType jobType, now time.Time) model.TimeBounds {
	var (
		fromTime *time.Time
		toTime   *time.Time
	)

	if jobType == jobTypeIncremental {
		fromTime = h.registry.GetRoutineState(h.routineName).LastRunTime.LatestRun()
	}

	if h.backupRoutine.BackupPolicy.IsSealedOrDefault() {
		toTime = &now
	}

	return model.TimeBounds{FromTime: fromTime, ToTime: toTime}
}

func (h *BackupRoutineOrchestrator) runIncrementalBackup(ctx context.Context, now time.Time) {
	if h.skipIncrementalBackup() {
		incrBackupSkippedCounter.Inc()
		return
	}

	duration, err := util.MeasureDuration(func() error {
		return h.runIncrementalBackupInternal(ctx, now)
	})
	if err != nil {
		incrBackupFailureCounter.Inc()
		h.logger.Error("Incremental backup failed", slog.Any("error", err))
	} else {
		incrBackupCounter.Inc()
		incrBackupDurationGauge.Set(float64(duration.Milliseconds()))
		h.logger.Debug("Finished incremental backup", slog.Int64("time", now.UnixMilli()))
	}
}

func (h *BackupRoutineOrchestrator) skipIncrementalBackup() bool {
	currentStat := h.registry.GetRoutineState(h.routineName)
	if currentStat.LastRunTime.NoFullBackup() {
		h.logger.Debug("Skip incremental backup until initial full backup is done")
		return true
	}
	if currentStat.Full != nil {
		h.logger.Debug("Full backup is currently in progress, skipping incremental backup")
		return true
	}
	if currentStat.Incremental != nil {
		h.logger.Debug("Incremental backup is currently in progress, skipping incremental backup")
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
		h.runner, client, namespaces, timeBounds, now, h.backupRoutine, jobTypeIncremental)
	h.registry.register(h.routineName, jobTypeIncremental, backupHandler)

	if err := backupHandler.Wait(ctx); err != nil {
		h.registry.remove(h.routineName, jobTypeIncremental)
		return err
	}

	go h.registry.unregister(h.routineName, jobTypeIncremental, now)

	return nil
}
