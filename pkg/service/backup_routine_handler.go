package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// BackupRoutineHandler implements backup logic for single routine.
type BackupRoutineHandler struct {
	backupService       Backup
	metadataWriter      backupMetadataManager
	backupFullPolicy    *model.BackupPolicy
	backupIncrPolicy    *model.BackupPolicy
	backupRoutine       *model.BackupRoutine
	routineName         string
	namespaces          []string
	storage             model.Storage
	secretAgent         *model.SecretAgent
	lastRun             lastBackupRun
	retry               executor
	clientManager       ClientManager
	logger              *slog.Logger
	clusterConfigWriter ClusterConfigWriter

	// backup handlers by namespace
	fullBackupHandlers map[string]BackupHandler
	incrBackupHandlers map[string]BackupHandler
}

// Backup represents a backup service.
type Backup interface {
	BackupRun(
		ctx context.Context,
		backupRoutine *model.BackupRoutine,
		backupPolicy *model.BackupPolicy,
		client *backup.Client,
		storage model.Storage,
		secretAgent *model.SecretAgent,
		timebounds model.TimeBounds,
		namespace string,
		path string,
	) (BackupHandler, error)
}

// BackupHandler represents a backup handler returned by the backup client.
type BackupHandler interface {
	// GetStats returns the statistics of the backup job.
	GetStats() *models.BackupStats
	// Wait waits for the backup job to complete and returns an error if the
	// job failed.
	Wait(context.Context) error
}

// backupMetadataManager handles backup metadata.
type backupMetadataManager interface {
	// writeBackupMetadata writes backup metadata to storage after successful backup.
	writeBackupMetadata(ctx context.Context, path string, metadata model.BackupMetadata) error
	// findLastRun scans storage for last backup time (on startup or config apply).
	findLastRun(ctx context.Context) lastBackupRun
}

// ClusterConfigWriter handles writing cluster configuration to storage.
type ClusterConfigWriter interface {
	Write(ctx context.Context, client backup.AerospikeClient, timestamp time.Time)
}

// BackupHandlerHolder stores backupHandlers by routine name
type BackupHandlerHolder map[string]*BackupRoutineHandler

// newBackupRoutineHandler returns a new BackupRoutineHandler instance.
func newBackupRoutineHandler(
	config *model.Config,
	clientManager ClientManager,
	backupService Backup,
	routineName string,
	backupBackend backupMetadataManager,
	lastRun lastBackupRun,
) *BackupRoutineHandler {
	backupRoutine := config.BackupRoutines[routineName]
	backupPolicy := backupRoutine.BackupPolicy
	backupStorage := backupRoutine.Storage
	logger := slog.Default().With(slog.String("routine", routineName))

	return &BackupRoutineHandler{
		backupService:    backupService,
		metadataWriter:   backupBackend,
		backupRoutine:    backupRoutine,
		backupFullPolicy: backupPolicy,
		backupIncrPolicy: backupPolicy.CopySMDDisabled(), // incremental backups should not contain metadata
		routineName:      routineName,
		namespaces:       backupRoutine.Namespaces,
		storage:          backupStorage,
		secretAgent:      backupRoutine.SecretAgent,
		lastRun:          lastRun,
		retry: newRetryExecutor(
			backupPolicy.GetRetryPolicyOrDefault(),
			logger),
		fullBackupHandlers: make(map[string]BackupHandler),
		incrBackupHandlers: make(map[string]BackupHandler),
		clientManager:      clientManager,
		clusterConfigWriter: NewClusterConfigWriter(
			backupStorage,
			routineName,
			backupPolicy,
			logger),
		logger: logger,
	}
}

func (h *BackupRoutineHandler) runFullBackup(ctx context.Context, now time.Time) {
	duration, err := util.MeasureDuration(func() error {
		return h.runFullBackupInternal(ctx, now)
	})

	if err != nil {
		h.logger.Error("Full backup failed", slog.Any("error", err))
		backupFailureCounter.Inc()
	} else {
		h.logger.Debug("Finished full backup")
		backupDurationGauge.Set(float64(duration.Milliseconds()))
		backupCounter.Inc()
	}
}

func (h *BackupRoutineHandler) runFullBackupInternal(ctx context.Context, now time.Time) error {
	client, namespaces, err := h.prepareCluster(h.retry)
	if err != nil {
		return err
	}

	defer func() {
		h.clientManager.Close(client)
		clear(h.fullBackupHandlers)
	}()

	for _, namespace := range namespaces {
		h.fullBackupHandlers[namespace] = h.startNamespaceBackup(ctx, namespace, now, client)
	}

	err = h.waitForFullBackups(ctx)
	if err != nil {
		return err
	}

	h.lastRun.full = now

	if h.backupFullPolicy.RemoveFiles.RemoveIncrementalBackup() {
		h.deleteFolder(ctx, getIncrementalRoot(h.routineName))
	}

	h.clusterConfigWriter.Write(ctx, client.AerospikeClient(), now)

	return nil
}

func (h *BackupRoutineHandler) prepareCluster(retry executor) (*backup.Client, []string, error) {
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
		namespaces, err = resolveNamespaces(h.namespaces, client.AerospikeClient())
		if err != nil {
			return fmt.Errorf("cannot retrieve namespaces from source cluster: %w", err)
		}

		return nil
	})

	return client, namespaces, err
}

func (h *BackupRoutineHandler) startNamespaceBackup(
	ctx context.Context, namespace string, now time.Time, client *backup.Client,
) BackupHandler {
	backupFolder := getFullPath(h.routineName, h.backupFullPolicy, namespace, now)
	timebounds := h.createTimebounds(true, now)

	return startBackup(
		ctx,
		h.retry,
		func(ctx context.Context) (BackupHandler, error) { // start backup.
			return h.backupService.BackupRun(
				ctx, h.backupRoutine, h.backupFullPolicy, client, h.storage, h.secretAgent, timebounds, namespace, backupFolder,
			)
		},
		func(ctx context.Context) { // on fail.
			h.deleteFolder(ctx, backupFolder)
		},
		func(ctx context.Context, stats *models.BackupStats) error { // on success.
			metadata := model.NewMetadataFromStats(stats, namespace, util.ValueOrZero(timebounds.FromTime), now)
			return h.writeBackupMetadata(ctx, metadata, backupFolder)
		},
	)
}

func (h *BackupRoutineHandler) createTimebounds(fullBackup bool, now time.Time) model.TimeBounds {
	var (
		fromTime *time.Time
		toTime   *time.Time
	)

	if !fullBackup {
		lastRun := h.lastRun.lastAnyRun()
		fromTime = &lastRun
	}

	if h.backupFullPolicy.IsSealed() {
		toTime = &now
	}

	return model.TimeBounds{FromTime: fromTime, ToTime: toTime}
}

func (h *BackupRoutineHandler) waitForFullBackups(ctx context.Context) error {
	var aggregatedErr error
	for ns, handler := range h.fullBackupHandlers {
		if err := handler.Wait(ctx); err != nil {
			aggregatedErr = errors.Join(aggregatedErr, fmt.Errorf("namespace %s: %w", ns, err))
		}
	}

	return aggregatedErr
}

func (h *BackupRoutineHandler) writeBackupMetadata(
	ctx context.Context, metadata model.BackupMetadata, backupFolder string,
) error {
	if err := h.metadataWriter.writeBackupMetadata(ctx, backupFolder, metadata); err != nil {
		h.logger.Error("Could not Write backup metadata",
			slog.String("folder", backupFolder),
			slog.Any("err", err))
		return err
	}

	return nil
}

func (h *BackupRoutineHandler) deleteFolder(ctx context.Context, path string) {
	err := storage.DeleteFolder(ctx, h.storage, path)
	if err != nil {
		h.logger.Error("Could not delete folder", slog.Any("err", err))
	}
	h.logger.Debug("Deleted folder", slog.String("path", path))
}

func (h *BackupRoutineHandler) runIncrementalBackup(ctx context.Context, now time.Time) {
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
		h.logger.Debug("Finished incremental backup")
	}
}

func (h *BackupRoutineHandler) skipIncrementalBackup() bool {
	if h.lastRun.noFullBackup() {
		h.logger.Debug("Skip incremental backup until initial full backup is done")
		return true
	}
	if len(h.fullBackupHandlers) > 0 {
		h.logger.Debug("Full backup is currently in progress, skipping incremental backup")
		return true
	}
	if len(h.incrBackupHandlers) > 0 {
		h.logger.Debug("Incremental backup is currently in progress, skipping incremental backup")
		return true
	}

	return false
}

func (h *BackupRoutineHandler) runIncrementalBackupInternal(ctx context.Context, now time.Time) error {
	client, namespaces, err := h.prepareCluster(&simpleExecutor{})
	if err != nil {
		return err
	}

	defer func() {
		h.clientManager.Close(client)
		clear(h.incrBackupHandlers)
	}()

	for _, namespace := range namespaces {
		h.incrBackupHandlers[namespace] = h.startIncrementalNamespaceBackup(ctx, namespace, now, client)
	}

	err = h.waitForIncrementalBackups(ctx, now)
	if err != nil {
		return err
	}

	h.lastRun.incremental = now
	return nil
}

func (h *BackupRoutineHandler) startIncrementalNamespaceBackup(
	ctx context.Context, namespace string, now time.Time, client *backup.Client,
) BackupHandler {
	backupFolder := getIncrementalPathForNamespace(h.routineName, namespace, now)
	timebounds := h.createTimebounds(false, now)

	return startBackup(
		ctx,
		&simpleExecutor{},
		func(ctx context.Context) (BackupHandler, error) { // start backup.
			return h.backupService.BackupRun(ctx,
				h.backupRoutine, h.backupIncrPolicy, client, h.storage, h.secretAgent,
				timebounds, namespace, backupFolder)
		},
		func(ctx context.Context) { // on fail.
			h.deleteFolder(ctx, backupFolder)
		},
		func(ctx context.Context, stats *models.BackupStats) error { // on success.
			if stats.IsEmpty() {
				return nil
			}

			metadata := model.NewMetadataFromStats(stats, namespace, util.ValueOrZero(timebounds.FromTime), now)
			return h.writeBackupMetadata(ctx, metadata, backupFolder)
		},
	)
}

func (h *BackupRoutineHandler) waitForIncrementalBackups(
	ctx context.Context, backupTimestamp time.Time,
) error {
	var aggregatedErr error
	for ns, handler := range h.incrBackupHandlers {
		err := handler.Wait(ctx)
		if err != nil {
			aggregatedErr = errors.Join(aggregatedErr, fmt.Errorf("namespace %s: %w", ns, err))
		}
	}

	return aggregatedErr
}

func (h *BackupRoutineHandler) GetCurrentStat() *model.CurrentBackups {
	return &model.CurrentBackups{
		Full:        currentBackupStatus(h.fullBackupHandlers),
		Incremental: currentBackupStatus(h.incrBackupHandlers),
	}
}
