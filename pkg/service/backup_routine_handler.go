package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// BackupRoutineHandler implements backup logic for single routine.
type BackupRoutineHandler struct {
	backupService    Backup
	backend          *BackupBackend
	backupFullPolicy *model.BackupPolicy
	backupIncrPolicy *model.BackupPolicy
	backupRoutine    *model.BackupRoutine
	routineName      string
	namespaces       []string
	storage          model.Storage
	secretAgent      *model.SecretAgent
	state            *model.BackupState
	retry            *RetryService
	clientManager    ClientManager

	// backup handlers by namespace
	fullBackupHandlers map[string]BackupHandler
	incrBackupHandlers map[string]BackupHandler
}

// BackupHandlerHolder stores backupHandlers by routine name
type BackupHandlerHolder map[string]*BackupRoutineHandler

// newBackupRoutineHandler returns a new BackupRoutineHandler instance.
func newBackupRoutineHandler(
	config *model.Config,
	clientManager ClientManager,
	backupService Backup,
	routineName string,
	backupBackend *BackupBackend,
) *BackupRoutineHandler {
	backupRoutine := config.BackupRoutines[routineName]
	storage := backupRoutine.Storage
	backupPolicy := backupRoutine.BackupPolicy
	secretAgent := backupRoutine.SecretAgent

	return &BackupRoutineHandler{
		backupService:      backupService,
		backend:            backupBackend,
		backupRoutine:      backupRoutine,
		backupFullPolicy:   backupPolicy,
		backupIncrPolicy:   backupPolicy.CopySMDDisabled(), // incremental backups should not contain metadata
		routineName:        routineName,
		namespaces:         backupRoutine.Namespaces,
		storage:            storage,
		secretAgent:        secretAgent,
		state:              backupBackend.readState(),
		retry:              NewRetryService(routineName),
		fullBackupHandlers: make(map[string]BackupHandler),
		incrBackupHandlers: make(map[string]BackupHandler),
		clientManager:      clientManager,
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
	logger := slog.Default().With(slog.String("routine", h.routineName))
	var err error
	if !h.backend.FullBackupInProgress().CompareAndSwap(false, true) {
		logger.Info("Full backup is currently in progress, skipping full backup")
		return nil
	}

	logger.Debug("Acquire fullBackupInProgress lock")
	defer h.backend.FullBackupInProgress().Store(false)

	client, err := h.clientManager.GetClient(h.backupRoutine.SourceCluster)
	if err != nil {
		return err
	}

	// release the lock
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
		h.deleteFolder(ctx, h.backend.incrementalBackupsPath, logger)
	}

	h.writeClusterConfiguration(ctx, client.AerospikeClient(), now)
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
		backupFolder := getFullPath(h.backend.fullBackupsPath, h.backupFullPolicy, namespace, upperBound)
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

func (h *BackupRoutineHandler) waitForFullBackups(ctx context.Context, backupTimestamp time.Time) error {
	startTime := time.Now() // startTime is only used to measure backup time
	for namespace, handler := range h.fullBackupHandlers {
		err := handler.Wait(ctx)
		if err != nil {
			backupFailureCounter.Inc()
			return fmt.Errorf("error during backup namespace %s, routine %s: %w",
				namespace, h.routineName, err)
		}

		backupFolder := getFullPath(h.backend.fullBackupsPath, h.backupFullPolicy, namespace, backupTimestamp)
		if err := h.writeBackupMetadata(ctx, handler.GetStats(), backupTimestamp, namespace, backupFolder); err != nil {
			return err
		}
	}
	backupDurationGauge.Set(float64(time.Since(startTime).Milliseconds()))
	return nil
}

func (h *BackupRoutineHandler) writeClusterConfiguration(
	ctx context.Context, client backup.AerospikeClient, now time.Time,
) {
	logger := slog.Default().With(slog.String("routine", h.routineName))

	infos := getClusterConfiguration(client)
	if len(infos) == 0 {
		logger.Warn("Could not read aerospike configuration")
		return
	}

	for i, info := range infos {
		confFilePath := getConfigurationFile(h, now, i)
		err := WriteFile(ctx, h.storage, confFilePath, []byte(info))
		if err != nil {
			logger.Error("Failed to Write configuration for the backup",
				slog.Any("err", err))
		}
	}
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

	if err := h.backend.writeBackupMetadata(ctx, backupFolder, metadata); err != nil {
		slog.Error("Could not Write backup metadata",
			slog.String("routine", h.routineName),
			slog.String("folder", backupFolder),
			slog.Any("err", err))
		return err
	}

	return nil
}

func (h *BackupRoutineHandler) deleteFolder(ctx context.Context, path string, logger *slog.Logger) {
	err := DeleteFolder(ctx, h.storage, path)
	if err != nil {
		logger.Error("Could not delete folder", slog.Any("err", err))
	}
}

func (h *BackupRoutineHandler) runIncrementalBackup(ctx context.Context, now time.Time) {
	logger := slog.Default().With(slog.String("routine", h.routineName))

	if h.state.LastFullRunIsEmpty() {
		logger.Debug("Skip incremental backup until initial full backup is done")
		return
	}
	if h.backend.FullBackupInProgress().Load() {
		logger.Debug("Full backup is currently in progress, skipping incremental backup")
		return
	}
	if len(h.incrBackupHandlers) > 0 {
		logger.Debug("Incremental backup is currently in progress, skipping incremental backup")
		return
	}

	client, err := h.clientManager.GetClient(h.backupRoutine.SourceCluster)
	if err != nil {
		logger.Error("cannot create backup client", slog.Any("err", err))
		return
	}

	defer func() {
		h.clientManager.Close(client)
		clear(h.incrBackupHandlers)
	}()

	h.startIncrementalBackupForAllNamespaces(ctx, client, now)

	h.waitForIncrementalBackups(ctx, now, logger)
	// increment incrBackupCounter metric
	incrBackupCounter.Inc()

	// update the state
	h.state.SetLastIncrRun(now)
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
		backupFolder := getIncrementalPathForNamespace(h.backend.incrementalBackupsPath, namespace, upperBound)
		handler, err := h.backupService.BackupRun(ctx,
			h.backupRoutine, h.backupIncrPolicy, client, h.storage, h.secretAgent,
			*timebounds, namespace, backupFolder)
		if err != nil {
			incrBackupFailureCounter.Inc()
			slog.Warn("could not start backup",
				slog.String("namespace", namespace),
				slog.String("routine", h.routineName),
				slog.Any("err", err))
		}
		h.incrBackupHandlers[namespace] = handler
	}
}

func (h *BackupRoutineHandler) waitForIncrementalBackups(
	ctx context.Context, backupTimestamp time.Time, logger *slog.Logger,
) {
	startTime := time.Now() // startTime is only used to measure backup time
	hasBackup := false
	for namespace, handler := range h.incrBackupHandlers {
		err := handler.Wait(ctx)
		if err != nil {
			slog.Warn("Failed incremental backup",
				slog.String("routine", h.routineName),
				slog.Any("err", err))
			incrBackupFailureCounter.Inc()
		}

		backupFolder := getIncrementalPathForNamespace(h.backend.incrementalBackupsPath, namespace, backupTimestamp)
		// delete if the backup file is empty
		if handler.GetStats().IsEmpty() {
			h.deleteFolder(ctx, backupFolder, logger)
			continue
		}
		if err := h.writeBackupMetadata(ctx, handler.GetStats(), backupTimestamp, namespace, backupFolder); err != nil {
			slog.Error("Could not Write backup metadata",
				slog.String("routine", h.routineName),
				slog.String("folder", backupFolder),
				slog.Any("err", err))
		}
		hasBackup = true
	}

	if !hasBackup {
		h.deleteFolder(ctx, getIncrementalPath(h.backend.incrementalBackupsPath, backupTimestamp), logger)
	}

	incrBackupDurationGauge.Set(float64(time.Since(startTime).Milliseconds()))
}

func (h *BackupRoutineHandler) GetCurrentStat() *model.CurrentBackups {
	return &model.CurrentBackups{
		Full:        currentBackupStatus(h.fullBackupHandlers),
		Incremental: currentBackupStatus(h.incrBackupHandlers),
	}
}
