package service

import (
	"context"
	"fmt"
	"log/slog"
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
	backend      BackupBackend
	backupPolicy *model.BackupPolicy
	cluster      *model.AerospikeCluster
	storage      *model.BackupStorage
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
		backupBackend = NewBackupBackendLocal(*storage.Path, *backupPolicy.Name)
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

// scheduleFullBackup runs the full backup periodically.
func (h *BackupHandler) scheduleFullBackup(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(*h.backupPolicy.IntervalMillis) * time.Millisecond)
	defer ticker.Stop()
loop:
	for {
		select {
		case now := <-ticker.C:
			if isStaleTick(now) {
				slog.Error("Skipped full backup", "name", *h.backupPolicy.Name)
				backupSkippedCounter.Inc()
				break
			}
			// read the state first and check
			state := h.backend.readState()
			if h.isFullEligible(now, state.LastRun) {
				backupRunFunc := func() {
					backupService.BackupRun(h.backupPolicy, h.cluster, h.storage, shared.BackupOptions{})
				}
				out := stdIO.Capture(backupRunFunc)
				util.LogCaptured(out)
				slog.Debug("Completed full backup", "name", *h.backupPolicy.Name)

				// increment backupCounter metric
				backupCounter.Inc()

				// update the state
				h.updateBackupState(now, state)
				// clean incremental backups
				h.backend.CleanDir(model.IncrementalBackupDirectory)
			} else {
				slog.Debug("The full backup is not due to run yet", "name", *h.backupPolicy.Name)
			}
		case <-ctx.Done():
			slog.Debug("ctx.Done in scheduleFullBackup")
			break loop
		}
	}
	slog.Info("Exiting scheduling loop for full backup", "name", *h.backupPolicy.Name)
}

// scheduleBackup runs the incremental backup periodically.
func (h *BackupHandler) scheduleIncrementalBackup(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(*h.backupPolicy.IncrIntervalMillis) * time.Millisecond)
	defer ticker.Stop()
loop:
	for {
		select {
		case now := <-ticker.C:
			if isStaleTick(now) {
				slog.Error("Skipped incremental backup", "name", *h.backupPolicy.Name)
				incrBackupSkippedCounter.Inc()
				break
			}
			// read the state first and check
			state := h.backend.readState()
			if h.isIncrementalEligible(now, state.LastIncrRun) {
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
				h.updateIncrementalBackupState(now, state)
			} else {
				slog.Debug("The incremental backup is not due to run yet", "name", *h.backupPolicy.Name)
			}
		case <-ctx.Done():
			slog.Debug("ctx.Done in scheduleIncrementalBackup")
			break loop
		}
	}
	slog.Info("Exiting scheduling loop for incremental backup", "name", *h.backupPolicy.Name)
}

// Schedule schedules backup for the defining policy.
func (h *BackupHandler) Schedule(ctx context.Context) {
	if h.backupPolicy.IncrIntervalMillis != nil && *h.backupPolicy.IncrIntervalMillis > 0 {
		slog.Info("Scheduling incremental backup", "name", *h.backupPolicy.Name)
		go h.scheduleIncrementalBackup(ctx)
	}
	slog.Info("Scheduling full backup", "name", *h.backupPolicy.Name)
	go h.scheduleFullBackup(ctx)
}

func (h *BackupHandler) isFullEligible(n time.Time, t time.Time) bool {
	return n.UnixMilli()-t.UnixMilli() >= *h.backupPolicy.IntervalMillis
}

func (h *BackupHandler) isIncrementalEligible(n time.Time, t time.Time) bool {
	return n.UnixMilli()-t.UnixMilli() >= *h.backupPolicy.IncrIntervalMillis
}

func (h *BackupHandler) updateBackupState(now time.Time, state *model.BackupState) {
	state.LastRun = now
	state.Performed++
	h.writeState(state)
}

func (h *BackupHandler) updateIncrementalBackupState(now time.Time, state *model.BackupState) {
	state.LastIncrRun = now
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
