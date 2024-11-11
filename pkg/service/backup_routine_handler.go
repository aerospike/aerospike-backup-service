package service

import (
	"context"
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
		backupService:      backupService,
		metadataWriter:     backupBackend,
		backupRoutine:      backupRoutine,
		backupFullPolicy:   backupPolicy,
		backupIncrPolicy:   backupPolicy.CopySMDDisabled(), // incremental backups should not contain metadata
		routineName:        routineName,
		namespaces:         backupRoutine.Namespaces,
		storage:            backupStorage,
		secretAgent:        backupRoutine.SecretAgent,
		state:              backupBackend.ReadState(),
		retry:              NewRetryService(logger),
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
	h.retry.retry(
		func() error { return h.runFullBackupInternal(ctx, now) },
		time.Duration(h.backupFullPolicy.GetRetryDelayOrDefault())*time.Millisecond,
		h.backupFullPolicy.GetMaxRetriesOrDefault(),
	)
}

func (h *BackupRoutineHandler) runFullBackupInternal(ctx context.Context, now time.Time) error {
	if len(h.fullBackupHandlers) > 0 {
		h.logger.Info("Full backup is currently in progress, skipping full backup")
		return nil
	}

	client, err := h.clientManager.GetClient(h.backupRoutine.SourceCluster)
	if err != nil {
		return err
	}

	defer func() {
		h.clientManager.Close(client)
		clear(h.fullBackupHandlers)
	}()

	err = h.startFullBackupForAllNamespaces(ctx, now, client)
	if err != nil {
		return err
	}

	err = h.waitForFullBackups(ctx, now)
	if err != nil {
		return err
	}

	// increment backupCounter metric
	backupCounter.Inc()

	// update the state
	h.state.SetLastFullRun(now)

	if h.backupFullPolicy.RemoveFiles.RemoveIncrementalBackup() {
		h.deleteFolder(ctx, getIncrementalRoot(h.routineName))
	}

	h.clusterConfigWriter.Write(ctx, client.AerospikeClient(), now)
	return nil
}

func (h *BackupRoutineHandler) startFullBackupForAllNamespaces(
	ctx context.Context, upperBound time.Time, client *backup.Client) error {
	clear(h.fullBackupHandlers)

	timebounds := model.TimeBounds{}
	if h.backupFullPolicy.IsSealed() {
		timebounds.ToTime = &upperBound
	}

	namespaces, err := getNamespacesToBackup(h.namespaces, client.AerospikeClient())
	if err != nil {
		return err
	}

	for _, namespace := range namespaces {
		backupFolder := getFullPath(h.routineName, h.backupFullPolicy, namespace, upperBound)
		handler, err := h.backupService.BackupRun(ctx, h.backupRoutine, h.backupFullPolicy, client,
			h.storage, h.secretAgent, timebounds, namespace, backupFolder)
		if err != nil {
			backupFailureCounter.Inc()
			return fmt.Errorf("could not start backup of namespace %s, routine %s: %w",
				namespace, h.routineName, err)
		}

		h.fullBackupHandlers[namespace] = handler
	}

	return nil
}

func (h *BackupRoutineHandler) waitForFullBackups(
	ctx context.Context, backupTimestamp time.Time,
) error {
	startTime := time.Now() // startTime is only used to measure backup time
	for namespace, handler := range h.fullBackupHandlers {
		backupFolder := getFullPath(h.routineName, h.backupFullPolicy, namespace, backupTimestamp)
		err := handler.Wait(ctx)
		if err != nil {
			backupFailureCounter.Inc()
			h.logger.Info("Delete failed backup folder",
				slog.String("path", backupFolder),
			)
			h.deleteFolder(ctx, backupFolder) // cleanup on failure
			return fmt.Errorf("error during backup namespace %s, routine %s: %w",
				namespace, h.routineName, err)
		}

		if err := h.writeBackupMetadata(ctx, handler.GetStats(), backupTimestamp, namespace, backupFolder); err != nil {
			return err
		}
	}
	duration := float64(time.Since(startTime).Milliseconds())
	h.logger.Debug("Finished full backup", slog.Float64("duration_ms", duration))
	backupDurationGauge.Set(duration)
	return nil
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
		// delete if the backup file is empty
		if handler.GetStats().IsEmpty() {
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
