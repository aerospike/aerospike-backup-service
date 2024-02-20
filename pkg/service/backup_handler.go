package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/aerospike/backup/pkg/stdio"
	"github.com/aerospike/backup/pkg/util"
)

// BackupHandler implements backup logic for single routine.
type BackupHandler struct {
	backend          *BackupBackend
	backupFullPolicy *model.BackupPolicy
	backupIncrPolicy *model.BackupPolicy
	backupRoutine    *model.BackupRoutine
	routineName      string
	cluster          *model.AerospikeCluster
	storage          *model.Storage
	secretAgent      *model.SecretAgent
	state            *model.BackupState
	timer            *time.Timer
}

// stdIO captures standard output
var stdIO = &stdio.CgoStdio{}

var backupService shared.Backup = shared.NewBackup()

// newBackupHandler returns a new BackupHandler instance.
func newBackupHandler(config *model.Config, routineName string, backupBackend *BackupBackend) (*BackupHandler, error) {
	backupRoutine := config.BackupRoutines[routineName]
	cluster := config.AerospikeClusters[backupRoutine.SourceCluster]
	storage := config.Storage[backupRoutine.Storage]
	backupPolicy := config.BackupPolicies[backupRoutine.BackupPolicy]
	var secretAgent *model.SecretAgent
	if backupRoutine.SecretAgent != nil {
		secretAgent = config.SecretAgents[*backupRoutine.SecretAgent]
	}

	if len(backupRoutine.Namespaces) == 0 {
		namespaces, err := getAllNamespacesOfCluster(cluster)
		if err != nil {
			return nil, fmt.Errorf("failed to get namespaces: %v", err)
		}
		backupRoutine.Namespaces = namespaces
	}

	return &BackupHandler{
		backend:          backupBackend,
		backupRoutine:    backupRoutine,
		backupFullPolicy: backupPolicy,
		backupIncrPolicy: backupPolicy.CopySMDDisabled(), // incremental backups should not contain metadata
		cluster:          cluster,
		storage:          storage,
		secretAgent:      secretAgent,
		state:            backupBackend.readState(),
		routineName:      routineName,
	}, nil
}

const maxRetries = 3
const retryInterval = 1 * time.Second

func (h *BackupHandler) runFullBackupWithRetry(now time.Time, n int) {
	if h.timer != nil {
		h.timer.Stop()
		if h.timer.C != nil {
			<-h.timer.C
		}
		h.timer = nil
	}

	err := h.runFullBackup(now)
	if err == nil {
		return
	}
	// log error
	slog.Warn("backup failed", "err", err)

	if n < maxRetries {
		h.timer = time.AfterFunc(retryInterval, func() {
			h.runFullBackupWithRetry(now, n+1)
		})
	}
}

// private method
func (h *BackupHandler) runFullBackup(now time.Time) error {
	if !h.backend.FullBackupInProgress().CompareAndSwap(false, true) {
		slog.Log(context.Background(), util.LevelTrace,
			"Full backup is currently in progress, skipping full backup",
			"name", h.routineName)
		return nil
	}
	slog.Debug("Acquire fullBackupInProgress lock", "name", h.routineName)
	// release the lock
	defer func() {
		h.backend.FullBackupInProgress().Store(false)
		slog.Debug("Release fullBackupInProgress lock", "name", h.routineName)
	}()
	for _, namespace := range h.backupRoutine.Namespaces {
		err := h.fullBackupForNamespace(now, namespace)
		if err != nil {
			return err
		}
	}

	// increment backupCounter metric
	backupCounter.Inc()

	// update the state
	h.updateFullBackupState(now)

	h.cleanIncrementalBackups()

	return nil
}

func (h *BackupHandler) fullBackupForNamespace(now time.Time, namespace string) error {
	backupFolder := getFullPath(h.backend.fullBackupsPath, h.backupFullPolicy, namespace, now)
	h.backend.CreateFolder(backupFolder)

	var stats *shared.BackupStat
	options := shared.BackupOptions{
		ModBefore: util.Ptr(now.UnixNano()),
	}
	backupRunFunc := func() {
		started := time.Now()
		backupPath := h.backend.wrapWithPrefix(backupFolder)
		stats = backupService.BackupRun(h.backupRoutine, h.backupFullPolicy, h.cluster,
			h.storage, h.secretAgent, options, &namespace, backupPath)
		if stats == nil {
			slog.Warn("Failed full backup", "name", h.routineName)
			backupFailureCounter.Inc()
			return
		}
		elapsed := time.Since(started)
		backupDurationGauge.Set(float64(elapsed.Milliseconds()))
	}
	slog.Debug("Starting full backup", "up to", now, "name", h.routineName)
	out := stdIO.Capture(backupRunFunc)
	slog.Debug("Completed full backup", "name", h.routineName)
	util.LogCaptured(out)

	if stats == nil {
		return fmt.Errorf("error during backup")
	}

	if err := h.backend.writeBackupMetadata(backupFolder, stats.ToModel(options, namespace)); err != nil {
		slog.Error("Could not write backup metadata", "name", h.routineName,
			"folder", backupFolder, "err", err)
		return err
	}
	return nil
}

func (h *BackupHandler) cleanIncrementalBackups() {
	if h.backupIncrPolicy.RemoveFiles.RemoveIncrementalBackup() {
		if err := h.backend.DeleteFolder(h.backend.incrementalBackupsPath); err != nil {
			slog.Error("Could not clean incremental backups", "name", h.routineName, "err", err)
		} else {
			slog.Info("Cleaned incremental backups", "name", h.routineName)
		}
	}
}

func (h *BackupHandler) runIncrementalBackup(now time.Time) {
	if h.state.LastFullRunIsEmpty() {
		slog.Log(context.Background(), util.LevelTrace,
			"Skip incremental backup until initial full backup is done",
			"name", h.routineName)
		return
	}
	if h.backend.FullBackupInProgress().Load() {
		slog.Log(context.Background(), util.LevelTrace,
			"Full backup is currently in progress, skipping incremental backup",
			"name", h.routineName)
		return
	}
	for _, namespace := range h.backupRoutine.Namespaces {
		h.runIncrBackupForNamespace(now, namespace)
	}

	// increment incrBackupCounter metric
	incrBackupCounter.Inc()

	// update the state
	h.updateIncrementalBackupState(now)
}

func (h *BackupHandler) runIncrBackupForNamespace(now time.Time, namespace string) {
	backupFolder := getIncrementalPath(h.backend.incrementalBackupsPath, namespace, now)
	h.backend.CreateFolder(backupFolder)

	var stats *shared.BackupStat
	options := shared.BackupOptions{
		ModBefore: util.Ptr(now.UnixNano()),
		ModAfter:  util.Ptr(h.state.LastRunEpoch()),
	}
	backupRunFunc := func() {
		started := time.Now()
		backupPath := h.backend.wrapWithPrefix(backupFolder)
		stats = backupService.BackupRun(
			h.backupRoutine, h.backupIncrPolicy, h.cluster, h.storage, h.secretAgent, options, &namespace, backupPath)
		if stats == nil {
			slog.Warn("Failed incremental backup", "name", h.routineName)
			incrBackupFailureCounter.Inc()
			return
		}
		elapsed := time.Since(started)
		incrBackupDurationGauge.Set(float64(elapsed.Milliseconds()))
	}
	slog.Debug("Starting incremental backup", "name", h.routineName)
	out := stdIO.Capture(backupRunFunc)
	slog.Debug("Completed incremental backup", "name", h.routineName)
	util.LogCaptured(out)
	// delete if the backup file is empty
	if h.isBackupEmpty(stats) {
		h.deleteEmptyBackup(backupFolder, h.routineName)
	} else {
		if err := h.backend.writeBackupMetadata(backupFolder, stats.ToModel(options, namespace)); err != nil {
			slog.Error("Could not write backup metadata", "name", h.routineName,
				"folder", backupFolder, "err", err)
		}
	}
}

func (h *BackupHandler) isBackupEmpty(stats *shared.BackupStat) bool {
	if stats == nil {
		return true
	}
	return stats.IsEmpty()
}

func (h *BackupHandler) deleteEmptyBackup(path string, routineName string) {
	if err := h.backend.DeleteFolder(path); err != nil {
		slog.Error("Failed to delete empty backup", "name", routineName,
			"path", path, "err", err)
	} else {
		slog.Debug("Deleted empty backup", "name", routineName, "path", path)
	}
}

func (h *BackupHandler) updateFullBackupState(now time.Time) {
	h.state.SetLastFullRun(now)
	h.writeState()
}

func (h *BackupHandler) updateIncrementalBackupState(now time.Time) {
	h.state.SetLastIncrRun(now)
	h.writeState()
}

func (h *BackupHandler) writeState() {
	if err := h.backend.writeState(h.state); err != nil {
		slog.Error("Failed to write state for the backup", "name", h.routineName, "err", err)
	}
}

func getFullPath(fullBackupsPath string, backupPolicy *model.BackupPolicy, namespace string, now time.Time) string {
	if backupPolicy.RemoveFiles.RemoveFullBackup() {
		path := fmt.Sprintf("%s/%s", fullBackupsPath, namespace)
		return path
	}
	path := fmt.Sprintf("%s/%s/%s", fullBackupsPath, timeSuffix(now), namespace)
	return path
}

func getIncrementalPath(incrBackupsPath string, namespace string, now time.Time) string {
	path := fmt.Sprintf("%s/%s/%s", incrBackupsPath, timeSuffix(now), namespace)
	return path
}

func timeSuffix(now time.Time) string {
	return strconv.FormatInt(now.UnixMilli(), 10)
}
