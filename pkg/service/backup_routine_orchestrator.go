package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// BackupRoutineOrchestrator orchestrates the execution of a single backup routine (both full and incremental).
// It manages all necessary preparations, executes the backup process, handles post-processing, and updates metrics.
type BackupRoutineOrchestrator struct {
	backupService       Backup
	backupFullPolicy    *model.BackupPolicy
	backupIncrPolicy    *model.BackupPolicy
	backupRoutine       *model.BackupRoutine
	namespaces          []string
	lastRun             *model.LastBackupRun
	retry               executor
	clientManager       aerospike.ClientManager
	logger              *slog.Logger
	clusterConfigWriter ClusterConfigWriter
	retentionManager    RetentionManager

	namespaceBackupRunner *BackupNamespaceRunner

	fullBackupHandler CancelableBackupHandler
	incrBackupHandler CancelableBackupHandler
}

var _ backupRunner = (*BackupRoutineOrchestrator)(nil)

// Backup is a facade for backup library.
type Backup interface {
	BackupRun(
		ctx context.Context,
		client *backup.Client,
		backupPolicy *model.BackupPolicy,
		timeBounds model.TimeBounds,
		namespace string,
		path string,
	) (BackupHandler, error)
}

// BackupHandler represents a backup handler for tracking and controlling backup operations.
// It is returned by the backup client library.
type BackupHandler interface {
	// GetStats returns the statistics of the backup job.
	GetStats() *models.BackupStats
	// Wait waits for the backup job to complete and returns an error if the job failed.
	Wait(context.Context) error
}

// CancelableBackupHandler extends BackupHandler with support for canceling the backup.
type CancelableBackupHandler interface {
	BackupHandler
	// Cancel cancels the backup operation.
	Cancel()
}

// ClusterConfigWriter handles writing cluster configuration to storage.
type ClusterConfigWriter interface {
	Write(ctx context.Context, client backup.AerospikeClient, timestamp time.Time)
}

// BackupHandlerHolder stores backupRunners by routine name
type BackupHandlerHolder = *util.SafeMap[string, backupRunner]

func NewBackupHandlerHolder() BackupHandlerHolder {
	return util.NewSafeMap[string, backupRunner]()
}

// newBackupRoutineHandler returns a new BackupRoutineOrchestrator instance.
func newBackupRoutineHandler(
	clientManager aerospike.ClientManager,
	backupService Backup,
	routineName string,
	routine *model.BackupRoutine,
	backupBackend BackupMetadataReaderWriter,
	lastRun *model.LastBackupRun,
) *BackupRoutineOrchestrator {
	backupPolicy := routine.BackupPolicy
	backupStorage := routine.Storage
	logger := slog.Default().With(slog.String("routine", routineName))
	retry := newRetryExecutor(
		backupPolicy.GetRetryPolicyOrDefault(),
		logger)
	return &BackupRoutineOrchestrator{
		namespaceBackupRunner: NewBackupNamespaceRunner(
			routineName,
			backupService,
			retry,
			backupBackend,
			logger,
		),

		backupService:    backupService,
		backupRoutine:    routine,
		backupFullPolicy: backupPolicy,
		backupIncrPolicy: backupPolicy.CopySMDDisabled(), // incremental backups should not contain metadata
		namespaces:       routine.Namespaces,
		lastRun:          lastRun,
		clientManager:    clientManager,
		clusterConfigWriter: NewClusterConfigWriter(
			backupStorage,
			routineName,
			backupPolicy,
			logger),
		logger: logger,
		retry:  retry,
		retentionManager: NewBackupRetentionManager(
			backupBackend, backupStorage, routineName, backupPolicy.RetentionPolicy),
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
	client, namespaces, err := h.prepareCluster(h.retry)
	if err != nil {
		return err
	}

	defer func() {
		h.clientManager.Close(client)
		h.fullBackupHandler = nil
	}()

	h.clusterConfigWriter.Write(ctx, client.AerospikeClient(), now)

	timeBounds := h.createTimebounds(true, now)
	h.fullBackupHandler = startNamespacesBackup(ctx,
		h.namespaceBackupRunner, client, namespaces, timeBounds, now, h.backupFullPolicy, jobTypeFull)

	if err = h.fullBackupHandler.Wait(ctx); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	h.lastRun.SetFullBackupTime(&now)

	go func() {
		err = h.retentionManager.deleteOldBackups(ctx)
		if err != nil {
			h.logger.Error("failed to clean up old backups", slog.Any("error", err))
		}
	}()

	return nil
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

func (h *BackupRoutineOrchestrator) createTimebounds(fullBackup bool, now time.Time) model.TimeBounds {
	var (
		fromTime *time.Time
		toTime   *time.Time
	)

	if !fullBackup {
		lastRun := h.lastRun.LatestRun()
		fromTime = lastRun
	}

	if h.backupFullPolicy.IsSealedOrDefault() {
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
	if h.lastRun.NoFullBackup() {
		h.logger.Debug("Skip incremental backup until initial full backup is done")
		return true
	}
	if h.fullBackupHandler != nil {
		h.logger.Debug("Full backup is currently in progress, skipping incremental backup")
		return true
	}
	if h.incrBackupHandler != nil {
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

	defer func() {
		h.clientManager.Close(client)
		h.incrBackupHandler = nil
	}()

	timeBounds := h.createTimebounds(false, now)
	h.incrBackupHandler = startNamespacesBackup(ctx,
		h.namespaceBackupRunner, client, namespaces, timeBounds, now, h.backupIncrPolicy, jobTypeIncremental)
	if err := h.incrBackupHandler.Wait(ctx); err != nil {
		return err
	}

	h.lastRun.SetIncrementalBackupTime(&now)
	return nil
}

func (h *BackupRoutineOrchestrator) CurrentStat() *model.CurrentBackups {
	return &model.CurrentBackups{
		Full:        currentBackupStatus(h.fullBackupHandler),
		Incremental: currentBackupStatus(h.incrBackupHandler),
		LastRunTime: h.lastRun,
	}
}

func (h *BackupRoutineOrchestrator) Cancel() {
	h.logger.Info("Canceling backup")
	if h.fullBackupHandler != nil {
		h.fullBackupHandler.Cancel()
	}
	if h.incrBackupHandler != nil {
		h.incrBackupHandler.Cancel()
	}
}
