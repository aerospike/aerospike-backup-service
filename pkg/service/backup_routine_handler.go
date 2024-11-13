package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/storage"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// BackupRoutineHandler implements backup logic for single routine.
type BackupRoutineHandler struct {
	backupService       Backup
	metadataWriter      BackupMetadataManager
	backupFullPolicy    *model.BackupPolicy
	backupIncrPolicy    *model.BackupPolicy
	backupRoutine       *model.BackupRoutine
	routineName         string
	namespaces          []string
	storage             model.Storage
	secretAgent         *model.SecretAgent
	state               *model.BackupState
	retry               *RetryService
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

// BackupMetadataManager handles backup metadata.
type BackupMetadataManager interface {
	// WriteBackupMetadata writes backup metadata to storage after successful backup.
	WriteBackupMetadata(ctx context.Context, path string, metadata model.BackupMetadata) error
	// ReadState scans storage for last backup state (on startup).
	ReadState() *model.BackupState
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
	backupBackend BackupMetadataManager,
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
		state:            backupBackend.ReadState(),
		retry: NewRetryService(
			time.Duration(backupPolicy.GetRetryDelayOrDefault())*time.Millisecond,
			int(backupPolicy.GetMaxRetriesOrDefault()),
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

func getNamespacesToBackup(namespaces []string, client backup.AerospikeClient) ([]string, error) {
	if len(namespaces) == 0 {
		return getAllNamespacesOfCluster(client)
	}

	return namespaces, nil
}

func (h *BackupRoutineHandler) runFullBackup(ctx context.Context, now time.Time) {
	if len(h.fullBackupHandlers) > 0 {
		h.logger.Info("Full backup is currently in progress, skipping full backup")
		return
	}

	client, namespaces, err := h.prepareClusterWithRetries()
	if err != nil {
		return
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
		return
	}

	// increment backupCounter metric
	backupCounter.Inc()

	// update the state
	h.state.SetLastFullRun(now)

	if h.backupFullPolicy.RemoveFiles.RemoveIncrementalBackup() {
		h.deleteFolder(ctx, getIncrementalRoot(h.routineName))
	}

	h.clusterConfigWriter.Write(ctx, client.AerospikeClient(), now)
}

func (h *BackupRoutineHandler) prepareClusterWithRetries() (*backup.Client, []string, error) {
	var (
		client     *backup.Client
		namespaces []string
	)

	err := h.retry.retry("cluster connection", func() error {
		var err error
		client, err = h.clientManager.GetClient(h.backupRoutine.SourceCluster)
		if err != nil {
			return fmt.Errorf("cannot get backup client: %w", err)
		}
		namespaces, err = getNamespacesToBackup(h.namespaces, client.AerospikeClient())
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
	timebounds := h.createTimebounds(now)

	return startRetryableBackup(
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
			return h.writeBackupMetadata(ctx, stats, now, namespace, backupFolder)
		},
	)
}

func (h *BackupRoutineHandler) createTimebounds(now time.Time) model.TimeBounds {
	if h.backupFullPolicy.IsSealed() {
		return *model.NewTimeBoundsTo(now)
	}
	return model.TimeBounds{}
}

func (h *BackupRoutineHandler) waitForFullBackups(ctx context.Context) error {
	startTime := time.Now()

	var aggregatedErr error
	for _, handler := range h.fullBackupHandlers {
		if err := handler.Wait(ctx); err != nil {
			backupFailureCounter.Inc()
			aggregatedErr = errors.Join(aggregatedErr, err)
		}
	}

	durationMs := float64(time.Since(startTime).Milliseconds())
	backupDurationGauge.Set(durationMs)

	if aggregatedErr == nil {
		h.logger.Debug("Finished full backup", slog.Float64("duration_ms", durationMs))
	}

	return aggregatedErr
}

func (h *BackupRoutineHandler) writeBackupMetadata(
	ctx context.Context, stats *models.BackupStats, created time.Time, namespace string, backupFolder string,
) error {
	metadata := model.BackupMetadata{
		From:                time.Time{},
		Created:             created,
		Namespace:           namespace,
		RecordCount:         stats.GetReadRecords(),
		FileCount:           stats.GetFileCount(),
		ByteCount:           stats.GetBytesWritten(),
		SecondaryIndexCount: uint64(stats.GetSIndexes()),
		UDFCount:            uint64(stats.GetUDFs()),
	}

	if err := h.metadataWriter.WriteBackupMetadata(ctx, backupFolder, metadata); err != nil {
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
}

func (h *BackupRoutineHandler) runIncrementalBackup(ctx context.Context, now time.Time) {
	if h.skipIncrementalBackup() {
		return
	}

	client, err := h.clientManager.GetClient(h.backupRoutine.SourceCluster)
	if err != nil {
		h.logger.Error("cannot create backup client", slog.Any("err", err))
		return
	}

	defer func() {
		h.clientManager.Close(client)
		clear(h.incrBackupHandlers)
	}()

	h.startIncrementalBackupForAllNamespaces(ctx, client, now)

	h.waitForIncrementalBackups(ctx, now)
	// increment incrBackupCounter metric
	incrBackupCounter.Inc()

	// update the state
	h.state.SetLastIncrRun(now)
}

func (h *BackupRoutineHandler) skipIncrementalBackup() bool {
	if h.state.LastFullRunIsEmpty() {
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

func (h *BackupRoutineHandler) startIncrementalBackupForAllNamespaces(
	ctx context.Context, client *backup.Client, upperBound time.Time) {
	timebounds := model.NewTimeBoundsFrom(h.state.LastRun())
	if h.backupFullPolicy.IsSealed() {
		timebounds.ToTime = &upperBound
	}

	clear(h.incrBackupHandlers)

	namespaces, err := getNamespacesToBackup(h.namespaces, client.AerospikeClient())
	if err != nil {
		return
	}

	for _, namespace := range namespaces {
		backupFolder := getIncrementalPathForNamespace(h.routineName, namespace, upperBound)
		handler, err := h.backupService.BackupRun(ctx,
			h.backupRoutine, h.backupIncrPolicy, client, h.storage, h.secretAgent,
			*timebounds, namespace, backupFolder)
		if err != nil {
			incrBackupFailureCounter.Inc()
			h.logger.Warn("could not start backup",
				slog.String("namespace", namespace),
				slog.Any("err", err))
			h.deleteFolder(ctx, backupFolder)
			continue
		}
		h.incrBackupHandlers[namespace] = handler
	}
}

func (h *BackupRoutineHandler) waitForIncrementalBackups(
	ctx context.Context, backupTimestamp time.Time,
) {
	startTime := time.Now() // startTime is only used to measure backup time
	hasBackup := false
	for namespace, handler := range h.incrBackupHandlers {
		err := handler.Wait(ctx)
		if err != nil {
			h.logger.Warn("Failed incremental backup",
				slog.Any("err", err))
			incrBackupFailureCounter.Inc()
		}

		backupFolder := getIncrementalPathForNamespace(h.routineName, namespace, backupTimestamp)
		// delete backup files if the backup is empty or failed
		if err != nil || handler.GetStats().IsEmpty() {
			h.deleteFolder(ctx, backupFolder)
			continue
		}
		if err := h.writeBackupMetadata(ctx, handler.GetStats(), backupTimestamp, namespace, backupFolder); err != nil {
			h.logger.Error("Could not Write backup metadata",
				slog.String("folder", backupFolder),
				slog.Any("err", err))
		}
		hasBackup = true
	}

	if !hasBackup {
		h.deleteFolder(ctx, getIncrementalTimestampPath(h.routineName, backupTimestamp))
	}

	incrBackupDurationGauge.Set(float64(time.Since(startTime).Milliseconds()))
}

func (h *BackupRoutineHandler) GetCurrentStat() *model.CurrentBackups {
	return &model.CurrentBackups{
		Full:        currentBackupStatus(h.fullBackupHandlers),
		Incremental: currentBackupStatus(h.incrBackupHandlers),
	}
}
