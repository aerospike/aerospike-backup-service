package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/aerospike/backup/pkg/stdio"
	"github.com/aerospike/backup/pkg/util"
)

// BackupScheduler knows how to schedule a backup.
type BackupScheduler interface {
	Schedule(ctx context.Context)
	GetBackend() BackupBackend
	BackupRoutineName() string
}

// BackupHandler handles a configured backup policy.
type BackupHandler struct {
	backend              BackupBackend
	backupFullPolicy     *model.BackupPolicy
	backupIncrPolicy     *model.BackupPolicy
	backupRoutine        *model.BackupRoutine
	routineName          string
	cluster              *model.AerospikeCluster
	storage              *model.Storage
	secretAgent          *model.SecretAgent
	state                *model.BackupState
	fullBackupInProgress *atomic.Bool
}

// stdIO captures standard output
var stdIO = &stdio.CgoStdio{}

var backupService shared.Backup = shared.NewBackup()

var _ BackupScheduler = (*BackupHandler)(nil)

var BackupScheduleTick = 1000 * time.Millisecond

// ScheduleHandlers schedules the configured backup policies.
func ScheduleHandlers(ctx context.Context, schedulers []BackupScheduler) {
	for _, scheduler := range schedulers {
		scheduler.Schedule(ctx)
	}
}

// BuildBackupSchedulers builds a list of BackupSchedulers according to
// the given configuration.
func BuildBackupSchedulers(config *model.Config) []BackupScheduler {
	schedulers := make([]BackupScheduler, 0, len(config.BackupPolicies))
	for routineName := range config.BackupRoutines {
		scheduler, err := newBackupHandler(config, routineName)
		if err != nil {
			panic(err)
		}
		schedulers = append(schedulers, scheduler)
	}
	return schedulers
}

// newBackupHandler returns a new BackupHandler instance.
func newBackupHandler(config *model.Config, routineName string) (*BackupHandler, error) {
	backupRoutine := config.BackupRoutines[routineName]
	cluster, found := config.AerospikeClusters[backupRoutine.SourceCluster]
	if !found {
		return nil, fmt.Errorf("cluster not found for %s", backupRoutine.SourceCluster)
	}
	storage, found := config.Storage[backupRoutine.Storage]
	if !found {
		return nil, fmt.Errorf("storage not found for %s", backupRoutine.Storage)
	}
	backupPolicy, found := config.BackupPolicies[backupRoutine.BackupPolicy]
	if !found {
		return nil, fmt.Errorf("backupPolicy not found for %s", backupRoutine.BackupPolicy)
	}
	var secretAgent *model.SecretAgent
	if backupRoutine.SecretAgent != nil {
		secretAgent = config.SecretAgents[*backupRoutine.SecretAgent]
	}

	fullBackupInProgress := &atomic.Bool{}
	backupBackend, err := newBackend(storage, backupPolicy, fullBackupInProgress)
	if err != nil {
		return nil, err
	}

	return &BackupHandler{
		backend:              backupBackend,
		backupRoutine:        backupRoutine,
		backupFullPolicy:     backupPolicy,
		backupIncrPolicy:     backupPolicy.CopySMDDisabled(), // incremental backups should not contain metadata
		cluster:              cluster,
		storage:              storage,
		secretAgent:          secretAgent,
		state:                backupBackend.readState(),
		fullBackupInProgress: fullBackupInProgress,
		routineName:          routineName,
	}, nil
}

func newBackend(
	storage *model.Storage,
	backupPolicy *model.BackupPolicy,
	fullBackupInProgress *atomic.Bool,
) (BackupBackend, error) {
	switch storage.Type {
	case model.Local:
		return NewBackupBackendLocal(storage, backupPolicy, fullBackupInProgress), nil
	case model.S3:
		return NewBackupBackendS3(storage, backupPolicy, fullBackupInProgress), nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %d. Should be one of:\n"+
			"\t%d for local storage\n"+
			"\t%d for AWS s3 compatible", storage.Type, model.Local, model.S3)
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
	if !h.fullBackupInProgress.CompareAndSwap(false, true) {
		slog.Log(context.Background(), util.LevelTrace,
			"Full backup is currently in progress, skipping full backup",
			"name", h.routineName)
		return
	}
	slog.Info("Run full backup", "name", h.routineName, "delta", time.Now().UnixMilli()-now.UnixMilli())
	slog.Debug("Acquire fullBackupInProgress lock", "name", h.routineName)
	// release the lock
	defer func() {
		h.fullBackupInProgress.Store(false)
		slog.Debug("Release fullBackupInProgress lock", "name", h.routineName)
	}()

	path := getPath(h.storage, h.backupFullPolicy, now)

	backupRunFunc := func() {
		started := time.Now()
		before := now.UnixNano()
		options := shared.BackupOptions{
			ModBefore: &before,
		}
		stats := backupService.BackupRun(h.backupRoutine, h.backupFullPolicy, h.cluster,
			h.storage, h.secretAgent, options, path, false)
		if stats == nil {
			slog.Warn("Failed full backup", "name", h.routineName)
			backupFailureCounter.Inc()
			return
		}
		elapsed := time.Since(started)
		backupDurationGauge.Set(float64(elapsed.Milliseconds()))
	}
	slog.Debug("Starting full backup", "name", h.routineName)
	out := stdIO.Capture(backupRunFunc)
	util.LogCaptured(out)
	slog.Debug("Completed full backup", "name", h.routineName)

	// increment backupCounter metric
	backupCounter.Inc()

	// update the state
	h.updateBackupState(now)

	// clean incremental backups
	if err := h.backend.CleanDir(model.IncrementalBackupDirectory); err != nil {
		slog.Error("could not clean incremental backups", "err", err)
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
	if h.fullBackupInProgress.Load() {
		slog.Log(context.Background(), util.LevelTrace,
			"Full backup is currently in progress, skipping incremental backup",
			"name", h.routineName)
		return
	}
	path := getIncrementalPath(h.storage, now)
	var stats *shared.BackupStat
	backupRunFunc := func() {
		lastRunEpoch := max(state.LastIncrRun.UnixNano(), state.LastFullRun.UnixNano())
		before := now.UnixNano()
		options := shared.BackupOptions{
			ModBefore: &before,
			ModAfter:  &lastRunEpoch,
		}
		started := time.Now()
		stats = backupService.BackupRun(h.backupRoutine, h.backupIncrPolicy, h.cluster,
			h.storage, h.secretAgent, options, path, true)
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
	h.deleteEmptyBackup(stats, h.routineName)

	// increment incrBackupCounter metric
	incrBackupCounter.Inc()

	// update the state
	h.updateIncrementalBackupState(now)
}

func (h *BackupHandler) deleteEmptyBackup(stats *shared.BackupStat, routineName string) {
	if stats == nil || !stats.HasStats {
		return
	}
	if stats.IsEmpty() {
		if err := h.backend.DeleteFile(stats.Path); err != nil {
			slog.Error("Failed to delete empty backup file", "name", routineName,
				"path", stats.Path, "err", err)
		} else {
			slog.Debug("Deleted empty backup file", "name", routineName, "path", stats.Path)
		}
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

// GetBackend returns the underlying BackupBackend.
func (h *BackupHandler) GetBackend() BackupBackend {
	return h.backend
}

// BackupRoutineName returns the name of the defining backup routine.
func (h *BackupHandler) BackupRoutineName() string {
	return h.routineName
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
	path := fmt.Sprintf("%s/%s/%s.asb", *storage.Path, model.IncrementalBackupDirectory, timeSuffix(now))
	return &path
}

func timeSuffix(now time.Time) string {
	return strconv.FormatInt(now.UnixMilli(), 10)
}
