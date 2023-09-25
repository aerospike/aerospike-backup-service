package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/backup/internal/util"
	"github.com/aerospike/backup/pkg/model"
)

const stateFileName = "state.json"

// BackupScheduler knows how to schedule a backup.
type BackupScheduler interface {
	ScheduleBackup(ctx context.Context)
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
		backupBackend = NewBackupBackendS3(storage, *backupPolicy.Name)
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

// ScheduleBackup runs the backup periodically.
func (h *BackupHandler) ScheduleBackup(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(*h.backupPolicy.IntervalMillis) * time.Millisecond)
	defer ticker.Stop()
loop:
	for {
		select {
		case <-ticker.C:
			// read the state first and check
			state := h.backend.readState()
			now := time.Now()
			if h.isEligible(now, state.LastRun) {
				backupRunFunc := func() {
					backupService.BackupRun(h.backupPolicy, h.cluster, h.storage)
				}
				out := util.CaptureStdout(backupRunFunc)
				slog.Debug("Completed backup", "name", *h.backupPolicy.Name, "out", out)

				// increment backupCounter metric
				backupCounter.Inc()

				// write the state
				h.writeState(now, state)
			} else {
				slog.Debug("The backup is not due to run yet", "name", *h.backupPolicy.Name)
			}
		case <-ctx.Done():
			slog.Debug("ctx.Done in ScheduleBackup")
			break loop
		}
	}
	slog.Info("Exiting scheduling loop for backup", "name", *h.backupPolicy.Name)
}

func (h *BackupHandler) isEligible(n time.Time, t time.Time) bool {
	return n.UnixMilli()-t.UnixMilli() >= *h.backupPolicy.IntervalMillis
}

func (h *BackupHandler) writeState(now time.Time, state *model.BackupState) {
	state.LastRun = now
	state.Performed++
	if err := h.backend.writeState(state); err != nil {
		slog.Error("Failed to write state for the backup", "name", *h.backupPolicy.Name)
	}
}

// GetBackend returns the underlying BackupBackend.
func (h *BackupHandler) GetBackend() BackupBackend {
	return h.backend
}
