package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

type HistoryManager interface {
	// FindLastRun finds the last backup run for a given routine.
	FindLastRun(ctx context.Context, routine *model.BackupRoutine) (*model.BackupTime, error)
}

// HistoryManagerImpl is a stateless service responsible for scanning
// storage backends to find the most recent backup timestamps.
type HistoryManagerImpl struct {
	backupReader BackupReader
}

// NewHistoryManager creates a new HistoryManagerImpl.
func NewHistoryManager(
	backupReader BackupReader,
) *HistoryManagerImpl {
	return &HistoryManagerImpl{
		backupReader: backupReader,
	}
}

// FindLastRun performs the I/O to find the last backup for a single routine.
func (hm *HistoryManagerImpl) FindLastRun(
	ctx context.Context,
	routine *model.BackupRoutine,
) (*model.BackupTime, error) {
	start := time.Now()
	fullStart := time.Now()
	lastFullBackup, err := hm.backupReader.GetBackups(ctx, NewFullBackupFilter(routine).Last())
	if err != nil {
		return nil, fmt.Errorf("read last full backup failed: %w", err)
	}
	slog.Debug("Last full backup scan completed",
		attr.Routine(routine.Name),
		slog.Int("backups", len(lastFullBackup)),
		slog.Duration("duration", time.Since(fullStart)),
	)

	if len(lastFullBackup) == 0 {
		slog.Debug("Last backup time scan completed for routine",
			attr.Routine(routine.Name),
			slog.String("lastRun", model.NewNoBackupTime().String()),
			slog.Duration("duration", time.Since(start)),
		)
		return model.NewNoBackupTime(), nil
	}
	lastFullTime := lastFullBackup[0].Created

	incrementalStart := time.Now()
	lastIncrBackup, err := hm.backupReader.GetBackups(ctx,
		NewIncrementalBackupFilter(routine).WithFromTime(lastFullTime).Last())
	if err != nil {
		return nil, fmt.Errorf("read last incremental backup failed: %w", err)
	}
	slog.Debug("Last incremental backup scan completed",
		attr.Routine(routine.Name),
		slog.Int("backups", len(lastIncrBackup)),
		slog.Duration("duration", time.Since(incrementalStart)),
	)

	var lastRun *model.BackupTime
	if len(lastIncrBackup) > 0 {
		lastRun = model.NewBackupTime(lastFullTime, lastIncrBackup[0].Created)
	} else {
		lastRun = model.NewFullBackupTime(lastFullTime)
	}

	slog.Debug("Last backup time scan completed for routine",
		attr.Routine(routine.Name),
		slog.String("lastRun", lastRun.String()),
		slog.Duration("duration", time.Since(start)))

	return lastRun, nil
}
