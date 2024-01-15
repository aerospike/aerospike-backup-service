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
	backend          BackupBackend
	backupFullPolicy *model.BackupPolicy
	backupIncrPolicy *model.BackupPolicy
	backupRoutine    *model.BackupRoutine
	routineName      string
	cluster          *model.AerospikeCluster
	storage          *model.Storage
	secretAgent      *model.SecretAgent
	state            *model.BackupState
}

// stdIO captures standard output
var stdIO = &stdio.CgoStdio{}

var backupService shared.Backup = shared.NewBackup()

// newBackupHandler returns a new BackupHandler instance.
func newBackupHandler(config *model.Config, routineName string, backupBackend BackupBackend) *BackupHandler {
	backupRoutine := config.BackupRoutines[routineName]
	cluster := config.AerospikeClusters[backupRoutine.SourceCluster]
	storage := config.Storage[backupRoutine.Storage]
	backupPolicy := config.BackupPolicies[backupRoutine.BackupPolicy]
	var secretAgent *model.SecretAgent
	if backupRoutine.SecretAgent != nil {
		secretAgent = config.SecretAgents[*backupRoutine.SecretAgent]
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
	}
}

func (h *BackupHandler) runFullBackup(now time.Time) {
	if !h.backend.FullBackupInProgress().CompareAndSwap(false, true) {
		slog.Log(context.Background(), util.LevelTrace,
			"Full backup is currently in progress, skipping full backup",
			"name", h.routineName)
		return
	}
	slog.Debug("Acquire fullBackupInProgress lock", "name", h.routineName)
	// release the lock
	defer func() {
		h.backend.FullBackupInProgress().Store(false)
		slog.Debug("Release fullBackupInProgress lock", "name", h.routineName)
	}()
	backupFolder := getFullPath(h.storage, h.backupFullPolicy, now)

	backupRunFunc := func() {
		started := time.Now()
		before := now.UnixNano()
		options := shared.BackupOptions{
			ModBefore: &before,
		}
		stats := backupService.BackupRun(h.backupRoutine, h.backupFullPolicy, h.cluster,
			h.storage, h.secretAgent, options, backupFolder)
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

	// increment backupCounter metric
	backupCounter.Inc()

	// update the state
	h.updateFullBackupState(now)

	if err := h.backend.writeBackupMetadata(*backupFolder, model.BackupMetadata{Created: now}); err != nil {
		slog.Error("Could not write backup metadata", "name", h.routineName,
			"folder", *backupFolder, "err", err)
	}

	// clean incremental backups
	if err := h.backend.CleanDir(model.IncrementalBackupDirectory); err != nil {
		slog.Error("Could not clean incremental backups", "name", h.routineName, "err", err)
	} else {
		slog.Info("Cleaned incremental backups", "name", h.routineName)
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
	backupFolder := getIncrementalPath(h.storage, now)

	var stats *shared.BackupStat
	backupRunFunc := func() {
		lastRunEpoch := h.state.LastRunEpoch()
		before := now.UnixNano()
		options := shared.BackupOptions{
			ModBefore: &before,
			ModAfter:  &lastRunEpoch,
		}
		started := time.Now()
		stats = backupService.BackupRun(
			h.backupRoutine, h.backupIncrPolicy, h.cluster, h.storage, h.secretAgent, options, backupFolder)
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
		h.deleteEmptyBackup(*backupFolder, h.routineName)
	} else {
		if err := h.backend.writeBackupMetadata(*backupFolder, model.BackupMetadata{Created: now}); err != nil {
			slog.Error("Could not write backup metadata", "name", h.routineName,
				"folder", *backupFolder, "err", err)
		}
	}

	// increment incrBackupCounter metric
	incrBackupCounter.Inc()

	// update the state
	h.updateIncrementalBackupState(now)
}

func (h *BackupHandler) isBackupEmpty(stats *shared.BackupStat) bool {
	if stats == nil || !stats.HasStats {
		return false
	}
	return stats.IsEmpty()
}

func (h *BackupHandler) deleteEmptyBackup(path string, routineName string) {
	if err := h.backend.DeleteFolder(path); err != nil {
		slog.Error("Failed to delete empty backup file", "name", routineName,
			"path", path, "err", err)
	} else {
		slog.Debug("Deleted empty backup file", "name", routineName, "path", path)
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

func getFullPath(storage *model.Storage, backupPolicy *model.BackupPolicy, now time.Time) *string {
	if backupPolicy.RemoveFiles != nil && !*backupPolicy.RemoveFiles {
		path := fmt.Sprintf("%s/%s/%s", *storage.Path, model.FullBackupDirectory, timeSuffix(now))
		return &path
	}
	path := fmt.Sprintf("%s/%s", *storage.Path, model.FullBackupDirectory)
	return &path
}

func getIncrementalPath(storage *model.Storage, now time.Time) *string {
	path := fmt.Sprintf("%s/%s/%s", *storage.Path, model.IncrementalBackupDirectory, timeSuffix(now))
	return &path
}

func timeSuffix(now time.Time) string {
	return strconv.FormatInt(now.UnixMilli(), 10)
}
