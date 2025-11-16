package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// HistoryManager is a stateless service responsible for scanning
// storage backends to find the most recent backup timestamps.
type HistoryManager struct {
	backupReader BackupReader
}

// NewHistoryManager creates a new HistoryManager.
func NewHistoryManager(
	backupReader BackupReader,
) *HistoryManager {
	return &HistoryManager{
		backupReader: backupReader,
	}
}

// FindLastRun performs the I/O to find the last backup for a single routine.
func (hm *HistoryManager) FindLastRun(
	ctx context.Context,
	routineName string,
) (*model.BackupTime, error) {
	lastFullBackup, err := hm.backupReader.GetBackups(ctx, NewFullBackupFilter(routineName).Last())
	if err != nil {
		return nil, fmt.Errorf("read last full backup failed: %w", err)
	}

	if len(lastFullBackup) == 0 {
		return model.NewNoBackupTime(), nil
	}
	lastFullTime := lastFullBackup[0].Created

	lastIncrBackup, err := hm.backupReader.GetBackups(ctx,
		NewIncrementalBackupFilter(routineName).WithFromTime(lastFullTime).Last())
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
		attr.Routine(routineName),
		slog.String("lastRun", lastRun.String()))

	return lastRun, nil
}
