package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/aerospike/aerospike-backup-service/internal/server/dto"
	"github.com/aerospike/aerospike-backup-service/pkg/model"
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
	storage          *model.Storage
	secretAgent      *model.SecretAgent
	state            *dto.BackupState
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

	err = h.waitForFullBackups(now)
	if err != nil {
		return err
	}

	// increment backupCounter metric
	backupCounter.Inc()

	// update the state
	h.updateFullBackupState(now)

	h.cleanIncrementalBackups()

	h.writeClusterConfiguration(client.AerospikeClient(), now)
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
		backupFolder := getFullPath(h.backend.fullBackupsPath, h.backupFullPolicy,
			namespace, upperBound)
		backupPath := h.backend.wrapWithPrefix(backupFolder)
		handler, err := h.backupService.BackupRun(ctx, h.backupRoutine, h.backupFullPolicy, client,
			h.storage, h.secretAgent, timebounds, namespace, backupPath)
		if err != nil {
			backupFailureCounter.Inc()
			return fmt.Errorf("could not start backup of namespace %s, routine %s: %w",
				namespace, h.routineName, err)
		}

		h.fullBackupHandlers[namespace] = handler
	}

	return nil
}

func (h *BackupRoutineHandler) waitForFullBackups(backupTimestamp time.Time) error {
	startTime := time.Now() // startTime is only used to measure backup time
	for namespace, handler := range h.fullBackupHandlers {
		err := handler.Wait()
		if err != nil {
			backupFailureCounter.Inc()
			return fmt.Errorf("error during backup namespace %s, routine %s: %w",
				namespace, h.routineName, err)
		}

		backupFolder := getFullPath(h.backend.fullBackupsPath, h.backupFullPolicy,
			namespace, backupTimestamp)
		if err := h.writeBackupMetadata(handler.GetStats(), backupTimestamp, namespace,
			backupFolder); err != nil {
			return err
		}
	}
	backupDurationGauge.Set(float64(time.Since(startTime).Milliseconds()))
	return nil
}

func (h *BackupRoutineHandler) writeClusterConfiguration(client backup.AerospikeClient, now time.Time) {
	logger := slog.Default().With(slog.String("routine", h.routineName))

	infos := getClusterConfiguration(client)
	if len(infos) == 0 {
		logger.Warn("Could not read aerospike configuration")
		return
	}

	path := getConfigurationPath(h.backend.fullBackupsPath, h.backupFullPolicy, now)
	for i, info := range infos {
		confFilePath := fmt.Sprintf("%s/aerospike_%d.conf", path, i)
		logger.Debug("Write aerospike configuration", slog.String("path", confFilePath))
		err := h.backend.write(confFilePath, []byte(info))
		if err != nil {
			logger.Error("Failed to write configuration for the backup",
				slog.Any("err", err))
		}
	}
}

func (h *BackupRoutineHandler) writeBackupMetadata(stats *models.BackupStats,
	created time.Time,
	namespace string,
	backupFolder string) error {
	metadata := dto.BackupMetadata{
		From:                time.Time{},
		Created:             created,
		Namespace:           namespace,
		RecordCount:         stats.GetReadRecords(),
		FileCount:           stats.GetFileCount(),
		ByteCount:           stats.GetBytesWritten(),
		SecondaryIndexCount: uint64(stats.GetSIndexes()),
		UDFCount:            uint64(stats.GetUDFs()),
	}

	if err := h.backend.writeBackupMetadata(backupFolder, metadata); err != nil {
		slog.Error("Could not write backup metadata",
			slog.String("routine", h.routineName),
			slog.String("folder", backupFolder),
			slog.Any("err", err))
		return err
	}

	return nil
}

func (h *BackupRoutineHandler) cleanIncrementalBackups() {
	if h.backupIncrPolicy.RemoveFiles.RemoveIncrementalBackup() {
		logger := slog.Default().With(slog.String("routine", h.routineName))
		if err := h.backend.DeleteFolder(h.backend.incrementalBackupsPath); err != nil {
			logger.Error("Could not clean incremental backups", slog.Any("err", err))
		} else {
			logger.Info("Cleaned incremental backups")
		}
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

	h.waitForIncrementalBackups(now)
	// increment incrBackupCounter metric
	incrBackupCounter.Inc()

	// update the state
	h.updateIncrementalBackupState(now)
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
		backupFolder := getIncrementalPathForNamespace(h.backend.incrementalBackupsPath,
			namespace, upperBound)
		backupPath := h.backend.wrapWithPrefix(backupFolder)
		handler, err := h.backupService.BackupRun(ctx,
			h.backupRoutine, h.backupIncrPolicy, client, h.storage, h.secretAgent,
			*timebounds, namespace, backupPath)
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

func (h *BackupRoutineHandler) waitForIncrementalBackups(backupTimestamp time.Time) {
	startTime := time.Now() // startTime is only used to measure backup time
	hasBackup := false
	for namespace, handler := range h.incrBackupHandlers {
		err := handler.Wait()
		if err != nil {
			slog.Warn("Failed incremental backup",
				slog.String("routine", h.routineName),
				slog.Any("err", err))
			incrBackupFailureCounter.Inc()
		}

		backupFolder := getIncrementalPathForNamespace(h.backend.incrementalBackupsPath,
			namespace, backupTimestamp)
		// delete if the backup file is empty
		if handler.GetStats().IsEmpty() {
			h.deleteEmptyBackup(backupFolder)
			continue
		}
		if err := h.writeBackupMetadata(handler.GetStats(), backupTimestamp, namespace,
			backupFolder); err != nil {
			slog.Error("Could not write backup metadata",
				slog.String("routine", h.routineName),
				slog.String("folder", backupFolder),
				slog.Any("err", err))
		}
		hasBackup = true
	}

	if !hasBackup {
		h.deleteEmptyBackup(getIncrementalPath(h.backend.incrementalBackupsPath,
			backupTimestamp))
	}

	incrBackupDurationGauge.Set(float64(time.Since(startTime).Milliseconds()))
}

func (h *BackupRoutineHandler) deleteEmptyBackup(path string) {
	if err := h.backend.DeleteFolder(path); err != nil {
		slog.Error("Failed to delete folder",
			slog.String("routine", h.routineName),
			slog.String("path", path),
			slog.Any("err", err))
	}
}

func (h *BackupRoutineHandler) updateFullBackupState(now time.Time) {
	h.state.SetLastFullRun(now)
	h.writeState()
}

func (h *BackupRoutineHandler) updateIncrementalBackupState(now time.Time) {
	h.state.SetLastIncrRun(now)
	h.writeState()
}

func (h *BackupRoutineHandler) writeState() {
	if err := h.backend.writeState(h.state); err != nil {
		slog.Error("Failed to write state for the backup",
			slog.String("routine", h.routineName),
			slog.Any("err", err))
	}
}

func getFullPath(fullBackupsPath string, backupPolicy *model.BackupPolicy, namespace string,
	now time.Time) string {
	if backupPolicy.RemoveFiles.RemoveFullBackup() {
		return fmt.Sprintf("%s/%s/%s", fullBackupsPath, dto.DataDirectory, namespace)
	}

	return fmt.Sprintf("%s/%s/%s/%s", fullBackupsPath, formatTime(now), dto.DataDirectory, namespace)
}

func getIncrementalPath(incrBackupsPath string, t time.Time) string {
	return fmt.Sprintf("%s/%s", incrBackupsPath, formatTime(t))
}

func getIncrementalPathForNamespace(incrBackupsPath string, namespace string, t time.Time) string {
	return fmt.Sprintf("%s/%s/%s", getIncrementalPath(incrBackupsPath, t), dto.DataDirectory, namespace)
}

func getConfigurationPath(fullBackupsPath string, backupPolicy *model.BackupPolicy, t time.Time) string {
	if backupPolicy.RemoveFiles.RemoveFullBackup() {
		path := fmt.Sprintf("%s/%s", fullBackupsPath, dto.ConfigurationBackupDirectory)
		return path
	}

	return fmt.Sprintf("%s/%s/%s", fullBackupsPath, formatTime(t), dto.ConfigurationBackupDirectory)
}

func formatTime(t time.Time) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}

func (h *BackupRoutineHandler) GetCurrentStat() *dto.CurrentBackups {
	return &dto.CurrentBackups{
		Full:        currentBackupStatus(h.fullBackupHandlers),
		Incremental: currentBackupStatus(h.incrBackupHandlers),
	}
}
