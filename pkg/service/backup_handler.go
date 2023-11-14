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
	storage              *model.BackupStorage
	fullBackupInProgress atomic.Bool
}

var _ BackupScheduler = (*BackupHandler)(nil)

// NewBackupHandler returns a new BackupHandler instance.
func NewBackupHandler(config *model.Config, backupPolicy *model.BackupPolicy) (*BackupHandler, error) {
	cluster, err := aerospikeClusterByName(*backupPolicy.SourceCluster, config.AerospikeClusters)
	if err != nil {
		return nil, err
	}
	storage, err := backupStorageByName(*backupPolicy.Storage, config.BackupStorage)
	if err != nil {
		return nil, err
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
	}, nil
}

// Schedule schedules backup for the defining policy.
func (h *BackupHandler) Schedule(ctx context.Context) {
	slog.Info("Scheduling full backup", "name", *h.backupPolicy.Name)
	h.scheduleBackupPeriodically(ctx, *h.backupPolicy.IntervalMillis, h.runFullBackup)

	if h.backupPolicy.IncrIntervalMillis != nil && *h.backupPolicy.IncrIntervalMillis > 0 {
		slog.Info("Scheduling incremental backup", "name", *h.backupPolicy.Name)
		h.scheduleBackupPeriodically(ctx, *h.backupPolicy.IncrIntervalMillis, h.runIncrementalBackup)
	}
}

// scheduleBackupPeriodically runs the backup periodically based on the provided interval.
func (h *BackupHandler) scheduleBackupPeriodically(
	ctx context.Context,
	intervalMillis int64,
	backupFunc func(time.Time)) {
	go func() {
		ticker := time.NewTicker(1000 * time.Millisecond)
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
	slog.Debug("Tick", "now", now, "name", *h.backupPolicy.Name)
	if isStaleTick(now) {
		slog.Error("Skipped full backup", "name", *h.backupPolicy.Name)
		backupSkippedCounter.Inc()
		return
	}
	// read the state first and check
	state := h.backend.readState()
	if !h.isFullEligible(now, state.LastRun) {
		slog.Debug("The full backup is not due to run yet", "name", *h.backupPolicy.Name)
		return
	}
	if !h.fullBackupInProgress.CompareAndSwap(false, true) {
		slog.Debug("Backup is currently in progress, skipping full backup", "name", *h.backupPolicy.Name)
		return
	}
	backupRunFunc := func() {
		backupService.BackupRun(h.backupPolicy, h.cluster, h.storage, shared.BackupOptions{})
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

	// release the lock
	h.fullBackupInProgress.Store(false)
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
		backupService.BackupRun(h.backupPolicy, h.cluster, h.storage, opts)
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
	state := h.backend.readState()
	state.LastRun = time.Now()
	state.Performed++
	h.writeState(state)
}

func (h *BackupHandler) updateIncrementalBackupState() {
	state := h.backend.readState()
	state.LastIncrRun = time.Now()
	h.writeState(state)
}

func (h *BackupHandler) writeState(state *model.BackupState) {
	if err := h.backend.writeState(state); err != nil {
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
