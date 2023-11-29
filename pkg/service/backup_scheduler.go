package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/aerospike/backup/pkg/stdio"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
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
	backupPolicy         *model.BackupPolicy
	backupRoutine        *model.BackupRoutine
	cluster              *model.AerospikeCluster
	storage              *model.Storage
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
	for _, backupRoutine := range config.BackupRoutines {
		scheduler, err := newBackupHandler(config, backupRoutine)
		if err != nil {
			panic(err)
		}
		schedulers = append(schedulers, scheduler)
	}
	return schedulers
}

// newBackupHandler returns a new BackupHandler instance.
func newBackupHandler(config *model.Config, backupRoutine *model.BackupRoutine) (*BackupHandler, error) {
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

	fullBackupInProgress := &atomic.Bool{}
	var backupBackend BackupBackend
	switch *storage.Type {
	case model.Local:
		backupBackend = NewBackupBackendLocal(*storage.Path, backupPolicy, fullBackupInProgress)
	case model.S3:
		backupBackend = NewBackupBackendS3(storage, backupPolicy, fullBackupInProgress)
	default:
		return nil, fmt.Errorf("unsupported storage type: %d", *storage.Type)
	}

	return &BackupHandler{
		backend:              backupBackend,
		backupRoutine:        backupRoutine,
		backupPolicy:         backupPolicy,
		cluster:              cluster,
		storage:              storage,
		state:                backupBackend.readState(),
		fullBackupInProgress: fullBackupInProgress,
	}, nil
}

// Schedule schedules backup for the defining policy.
func (h *BackupHandler) Schedule(ctx context.Context) {
	slog.Info("Scheduling full backup", "name", h.backupRoutine.Name)
	h.scheduleBackupPeriodically(ctx, h.runFullBackup)

	if h.backupRoutine.IncrIntervalMillis != nil && *h.backupRoutine.IncrIntervalMillis > 0 {
		slog.Info("Scheduling incremental backup", "name", h.backupRoutine.Name)
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
		slog.Debug("Skipped full backup", "name", h.backupRoutine.Name)
		backupSkippedCounter.Inc()
		return
	}
	if !h.fullBackupInProgress.CompareAndSwap(false, true) {
		slog.Debug("Full backup is currently in progress, skipping full backup", "name", h.backupRoutine.Name)
		return
	}
	slog.Debug("Acquire fullBackupInProgress lock", "name", h.backupRoutine.Name)
	// release the lock
	defer func() {
		h.fullBackupInProgress.Store(false)
		slog.Debug("Release fullBackupInProgress lock", "name", h.backupRoutine.Name)
	}()

	if !h.isFullEligible(now, h.state.LastRun) {
		slog.Debug("The full backup is not due to run yet", "name", h.backupRoutine.Name)
		return
	}
	backupRunFunc := func() {
		started := time.Now()
		if !backupService.BackupRun(h.backupRoutine, h.backupPolicy, h.cluster,
			h.storage, shared.BackupOptions{}) {
			backupFailureCounter.Inc()
		} else {
			elapsed := time.Since(started)
			backupDurationGauge.Set(float64(elapsed.Milliseconds()))
		}
	}
	out := stdIO.Capture(backupRunFunc)
	util.LogCaptured(out)
	slog.Debug("Completed full backup", "name", h.backupRoutine.Name)

	// increment backupCounter metric
	backupCounter.Inc()

	// update the state
	h.updateBackupState()

	// clean incremental backups
	h.backend.CleanDir(model.IncrementalBackupDirectory)
}

func (h *BackupHandler) runIncrementalBackup(now time.Time) {
	if isStaleTick(now) {
		slog.Error("Skipped incremental backup", "name", h.backupRoutine.Name)
		incrBackupSkippedCounter.Inc()
		return
	}
	// read the state first and check
	state := h.backend.readState()
	if state.LastRun == (time.Time{}) {
		slog.Debug("Skip incremental backup until initial full backup is done", "name", h.backupRoutine.Name)
		return
	}
	if !h.isIncrementalEligible(now, state.LastIncrRun) {
		slog.Debug("The incremental backup is not due to run yet", "name", h.backupRoutine.Name)
		return
	}
	if h.fullBackupInProgress.Load() {
		slog.Debug("Full backup is currently in progress, skipping incremental backup", "name", h.backupRoutine.Name)
		return
	}
	backupRunFunc := func() {
		opts := shared.BackupOptions{}
		lastIncrRunEpoch := state.LastIncrRun.UnixNano()
		opts.ModAfter = &lastIncrRunEpoch
		started := time.Now()
		if !backupService.BackupRun(h.backupRoutine, h.backupPolicy, h.cluster, h.storage, opts) {
			incrBackupFailureCounter.Inc()
		} else {
			elapsed := time.Since(started)
			incrBackupDurationGauge.Set(float64(elapsed.Milliseconds()))
		}
	}
	out := stdIO.Capture(backupRunFunc)
	util.LogCaptured(out)
	slog.Debug("Completed incremental backup", "name", h.backupRoutine.Name)
	stats, _ := util.ExtractBackupStats(out)

	h.deleteEmptyBackup(stats)

	// increment incrBackupCounter metric
	incrBackupCounter.Inc()

	// update the state
	h.updateIncrementalBackupState()
}

func (h *BackupHandler) deleteEmptyBackup(stats *util.BackupInfo) {
	if stats != nil && stats.RecordCount == 0 && stats.Path != "" {
		h.backend.DeleteFile(stats.Path)
	}
}

func (h *BackupHandler) isFullEligible(n time.Time, t time.Time) bool {
	return n.UnixMilli()-t.UnixMilli() >= *h.backupRoutine.IntervalMillis
}

func (h *BackupHandler) isIncrementalEligible(n time.Time, t time.Time) bool {
	return n.UnixMilli()-t.UnixMilli() >= *h.backupRoutine.IncrIntervalMillis
}

func (h *BackupHandler) updateBackupState() {
	h.state.LastRun = time.Now()
	h.state.Performed++
	h.writeState()
}

func (h *BackupHandler) updateIncrementalBackupState() {
	h.state.LastIncrRun = time.Now()
	h.writeState()
}

func (h *BackupHandler) writeState() {
	if err := h.backend.writeState(h.state); err != nil {
		slog.Error("Failed to write state for the backup", "name", h.backupRoutine.Name, "err", err)
	}
}

// GetBackend returns the underlying BackupBackend.
func (h *BackupHandler) GetBackend() BackupBackend {
	return h.backend
}

// BackupRoutineName returns the name of the defining backup routine.
func (h *BackupHandler) BackupRoutineName() string {
	return h.backupRoutine.Name
}

func isStaleTick(t time.Time) bool {
	return time.Since(t) > time.Second
}
