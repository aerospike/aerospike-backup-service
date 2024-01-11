package service

import (
	"context"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/aerospike/backup/pkg/stdio"
	"github.com/aerospike/backup/pkg/util"
)

// BackupHandler handles a configured backup policy.
type BackupHandler struct {
	backend          BackupBackend
	backupListReader BackupListReader
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

var BackupScheduleTick = 1000 * time.Millisecond

// BuildBackupSchedulers builds a list of BackupSchedulers according to
// the given configuration.
func BuildBackupSchedulers(ctx context.Context, config *model.Config, backends map[string]BackupBackend) {
	for routineName := range config.BackupRoutines {
		handler := newBackupHandler(config, routineName, backends[routineName])
		handler.Schedule(ctx)
	}
}

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

// Schedule schedules backup for the defining policy.
func (h *BackupHandler) Schedule(ctx context.Context) {
	slog.Info("Scheduling full backup", "name", h.routineName)
	h.scheduleBackupPeriodically(ctx, h.runFullBackup)

	if h.backupRoutine.IncrIntervalMillis != nil {
		slog.Info("Scheduling incremental backup", "name", h.routineName)
		h.scheduleBackupPeriodically(ctx, h.runIncrementalBackup)
	}
}

// scheduleBackupPeriodically runs the backup periodically based on the provided interval.
func (h *BackupHandler) scheduleBackupPeriodically(
	ctx context.Context,
	backupFunc func(time.Time),
) {
	go func() {
		ticker := time.NewTicker(BackupScheduleTick)
		defer ticker.Stop()
		for {
			select {
			case now := <-ticker.C:
				backupFunc(now)
			case <-ctx.Done():
				slog.Debug("ctx.Done in scheduleBackupPeriodically")
				return
			}
		}
	}()
	// Run the backup immediately
	go backupFunc(time.Now())
}

func (h *BackupHandler) runFullBackup(now time.Time) {
	if isStaleTick(now) {
		slog.Debug("Skipped full backup", "name", h.routineName)
		backupSkippedCounter.Inc()
		return
	}
	if !h.isFullEligible(now, h.state.LastFullRun) {
		slog.Log(context.Background(), util.LevelTrace,
			"The full backup is not due to run yet", "name", h.routineName)
		return
	}
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

	backupFolder := getPath(h.storage, h.backupFullPolicy, now)

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
	util.LogCaptured(out)
	slog.Debug("Completed full backup", "name", h.routineName)

	// increment backupCounter metric
	backupCounter.Inc()

	// update the state
	h.updateBackupState(now)

	if err := h.backend.writeBackupMetadata(*backupFolder, model.BackupMetadata{Created: now}); err != nil {
		slog.Error("could not write metadata", "folder", *backupFolder, "err", err)
	}

	// clean incremental backups
	if err := h.backend.CleanDir(model.IncrementalBackupDirectory); err != nil {
		slog.Error("could not clean incremental backups", "err", err)
	} else {
		slog.Info("cleaned incr backups", "name", h.routineName)
	}
}

func (h *BackupHandler) runIncrementalBackup(now time.Time) {
	if isStaleTick(now) {
		slog.Debug("Skipped incremental backup", "name", h.routineName)
		incrBackupSkippedCounter.Inc()
		return
	}
	// read the state first and check
	state := h.backend.readState()
	if state.LastFullRun == (time.Time{}) {
		slog.Log(context.Background(), util.LevelTrace,
			"Skip incremental backup until initial full backup is done",
			"name", h.routineName)
		return
	}
	if !h.isIncrementalEligible(now, state.LastIncrRun, state.LastFullRun) {
		slog.Log(context.Background(), util.LevelTrace,
			"The incremental backup is not due to run yet", "name", h.routineName)
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
		lastRunEpoch := max(state.LastIncrRun.UnixNano(), state.LastFullRun.UnixNano())
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
	util.LogCaptured(out)
	slog.Debug("Completed incremental backup", "name", h.routineName)
	// delete if the backup file is empty
	if h.isBackupEmpty(stats) {
		h.deleteEmptyBackup(*backupFolder, h.routineName)
	} else {
		if err := h.backend.writeBackupMetadata(*backupFolder, model.BackupMetadata{Created: now}); err != nil {
			slog.Error("could not write metadata", "folder", *backupFolder, "err", err)
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

func (h *BackupHandler) isFullEligible(now time.Time, lastFullRun time.Time) bool {
	return now.UnixMilli()-lastFullRun.UnixMilli() >= *h.backupRoutine.IntervalMillis
}

func (h *BackupHandler) isIncrementalEligible(now time.Time, lastIncrRun time.Time, lastFullRun time.Time) bool {
	if now.UnixMilli()-lastFullRun.UnixMilli() < *h.backupRoutine.IncrIntervalMillis {
		return false // Full backup happened recently
	}

	return now.UnixMilli()-lastIncrRun.UnixMilli() >= *h.backupRoutine.IncrIntervalMillis
}

func (h *BackupHandler) updateBackupState(now time.Time) {
	h.state.LastFullRun = now
	h.state.Performed++
	h.writeState()
}

func (h *BackupHandler) updateIncrementalBackupState(now time.Time) {
	h.state.LastIncrRun = now
	h.writeState()
}

func (h *BackupHandler) writeState() {
	if err := h.backend.writeState(h.state); err != nil {
		slog.Error("Failed to write state for the backup", "name", h.routineName, "err", err)
	}
}

func isStaleTick(t time.Time) bool {
	return time.Since(t) > time.Second
}
func getPath(storage *model.Storage, backupPolicy *model.BackupPolicy, now time.Time) *string {
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
