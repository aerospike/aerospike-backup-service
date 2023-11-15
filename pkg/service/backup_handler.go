package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/aerospike/backup/pkg/util"
)

// BackupScheduler knows how to schedule a backup.
type BackupScheduler interface {
	Schedule(ctx context.Context)
	GetBackend() BackupBackend
}

// BackupHandler handles a configured backup policy.
type BackupHandler struct {
	backend              BackupBackend
	backupPolicy         *model.BackupPolicy
	cluster              *model.AerospikeCluster
	storage              *model.Storage
	state                *model.BackupState
	fullBackupInProgress atomic.Bool
}

var _ BackupScheduler = (*BackupHandler)(nil)

var BackupScheduleTick = 1000 * time.Millisecond

// NewBackupHandler returns a new BackupHandler instance.
func NewBackupHandler(config *model.Config, backupPolicy *model.BackupPolicy) (*BackupHandler, error) {
	cluster, found := config.AerospikeClusters[*backupPolicy.SourceCluster]
	if !found {
		return nil, fmt.Errorf("cluster not found for %s", *backupPolicy.SourceCluster)
	}
	storage, found := config.Storage[*backupPolicy.Storage]
	if !found {
		return nil, fmt.Errorf("storage not found for %s", *backupPolicy.Storage)
	}

	var backupBackend BackupBackend
	switch *storage.Type {
	case model.Local:
		backupBackend = NewBackupBackendLocal(*storage.Path, backupPolicy)
	case model.S3:
		backupBackend = NewBackupBackendS3(storage, backupPolicy)
	default:
		return nil, fmt.Errorf("unsupported storage type: %d", *storage.Type)
	}

	return &BackupHandler{
		backend:      backupBackend,
		backupPolicy: backupPolicy,
		cluster:      cluster,
		storage:      storage,
		state:        backupBackend.readState(),
	}, nil
}

// Schedule schedules backup for the defining policy.
func (h *BackupHandler) Schedule(ctx context.Context) {
	slog.Info("Scheduling full backup", "name", *h.backupPolicy.Name)
	h.scheduleBackupPeriodically(ctx, h.runFullBackup)

	if h.backupPolicy.IncrIntervalMillis != nil && *h.backupPolicy.IncrIntervalMillis > 0 {
		slog.Info("Scheduling incremental backup", "name", *h.backupPolicy.Name)
		h.scheduleBackupPeriodically(ctx, h.runIncrementalBackup)
	}
}

// scheduleBackupPeriodically runs the backup periodically based on the provided interval.
func (h *BackupHandler) scheduleBackupPeriodically(
	ctx context.Context,
	backupFunc func(time.Time)) {
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
		slog.Debug("Skipped full backup", "name", *h.backupPolicy.Name)
		backupSkippedCounter.Inc()
		return
	}
	if !h.fullBackupInProgress.CompareAndSwap(false, true) {
		slog.Debug("Backup is currently in progress, skipping full backup", "name", *h.backupPolicy.Name)
		return
	}
	// release the lock
	defer h.fullBackupInProgress.Store(false)

	if !h.isFullEligible(now, h.state.LastRun) {
		slog.Debug("The full backup is not due to run yet", "name", *h.backupPolicy.Name)
		return
	}
	backupRunFunc := func() {
		started := time.Now()
		if !backupService.BackupRun(h.backupPolicy, h.cluster, h.storage, shared.BackupOptions{}) {
			backupFailureCounter.Inc()
		} else {
			elapsed := time.Since(started)
			backupDurationGauge.Set(float64(elapsed.Milliseconds()))
		}
	}
	out := stdIO.Capture(backupRunFunc)
	util.LogCaptured(out)
	slog.Debug("Completed full backup", "name", *h.backupPolicy.Name)

	// increment backupCounter metric
	backupCounter.Inc()

	// update the state
	h.updateBackupState()

	// clean incremental backups
	h.backend.CleanDir(model.IncrementalBackupDirectory)
}

func (h *BackupHandler) runIncrementalBackup(now time.Time) {
	if isStaleTick(now) {
		slog.Error("Skipped incremental backup", "name", *h.backupPolicy.Name)
		incrBackupSkippedCounter.Inc()
		return
	}
	// read the state first and check
	state := h.backend.readState()
	if state.LastRun == (time.Time{}) {
		slog.Debug("Skip incremental backup until initial full backup is done", "name", *h.backupPolicy.Name)
		return
	}
	if !h.isIncrementalEligible(now, state.LastIncrRun) {
		slog.Debug("The incremental backup is not due to run yet", "name", *h.backupPolicy.Name)
		return
	}
	if h.fullBackupInProgress.Load() {
		slog.Debug("Full backup is currently in progress, skipping incremental backup", "name", *h.backupPolicy.Name)
		return
	}
	backupRunFunc := func() {
		opts := shared.BackupOptions{}
		lastIncrRunEpoch := state.LastIncrRun.UnixNano()
		opts.ModAfter = &lastIncrRunEpoch
		started := time.Now()
		if !backupService.BackupRun(h.backupPolicy, h.cluster, h.storage, opts) {
			incrBackupFailureCounter.Inc()
		} else {
			elapsed := time.Since(started)
			incrBackupDurationGauge.Set(float64(elapsed.Milliseconds()))
		}
	}
	out := stdIO.Capture(backupRunFunc)
	util.LogCaptured(out)
	slog.Debug("Completed incremental backup", "name", *h.backupPolicy.Name)

	// increment incrBackupCounter metric
	incrBackupCounter.Inc()

	// update the state
	h.updateIncrementalBackupState()
}

func (h *BackupHandler) isFullEligible(n time.Time, t time.Time) bool {
	return n.UnixMilli()-t.UnixMilli() >= *h.backupPolicy.IntervalMillis
}

func (h *BackupHandler) isIncrementalEligible(n time.Time, t time.Time) bool {
	return n.UnixMilli()-t.UnixMilli() >= *h.backupPolicy.IncrIntervalMillis
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
		slog.Error("Failed to write state for the backup", "name", *h.backupPolicy.Name, "err", err)
	}
}

// GetBackend returns the underlying BackupBackend.
func (h *BackupHandler) GetBackend() BackupBackend {
	return h.backend
}

func isStaleTick(t time.Time) bool {
	return time.Since(t) > time.Second
}
