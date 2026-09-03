package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// HistoryManager finds the last full and incremental backup times of a routine in storage.
type HistoryManager interface {
	// FindLastRun finds the last backup run for a given routine.
	FindLastRun(ctx context.Context, routine *model.BackupRoutine) (*model.BackupTime, error)
}

type historyManager struct {
	backupReader BackupReader
}

var _ HistoryManager = (*historyManager)(nil)

// NewHistoryManager returns a HistoryManager.
func NewHistoryManager(
	backupReader BackupReader,
) HistoryManager {
	return &historyManager{
		backupReader: backupReader,
	}
}

// FindLastRun performs the I/O to find the last backup for a single routine.
func (hm *historyManager) FindLastRun(
	ctx context.Context,
	routine *model.BackupRoutine,
) (*model.BackupTime, error) {
	lastFullBackup, err := hm.backupReader.GetBackups(ctx, NewFullBackupFilter(routine).Last())
	if err != nil {
		return nil, fmt.Errorf("read last full backup failed: %w", err)
	}

	if len(lastFullBackup) == 0 {
		return model.NewNoBackupTime(), nil
	}
	lastFullTime := lastFullBackup[0].Created

	lastIncrBackup, err := hm.backupReader.GetBackups(ctx,
		NewIncrementalBackupFilter(routine).WithFromTime(lastFullTime).Last())
	if err != nil {
		return nil, fmt.Errorf("read last incremental backup failed: %w", err)
	}

	var lastRun *model.BackupTime
	if len(lastIncrBackup) > 0 {
		lastRun = model.NewBackupTime(lastFullTime, lastIncrBackup[0].Created)
	} else {
		lastRun = model.NewFullBackupTime(lastFullTime)
	}

	slog.Debug("Last backup time scan completed for routine",
		attr.Routine(routine.Name),
		slog.String("lastRun", lastRun.String()))

	return lastRun, nil
}
