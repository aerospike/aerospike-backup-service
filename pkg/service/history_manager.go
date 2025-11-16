package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/google/martian/v3"
	"golang.org/x/sync/errgroup"
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

// FindLastRunBatch finds the last backup for each of the given routines in parallel.
func (hm *HistoryManager) FindLastRunBatch(
	ctx context.Context,
	routineNames []string,
) (map[string]*model.BackupTime, error) {
	results := make(map[string]*model.BackupTime, len(routineNames))
	mu := &sync.Mutex{}
	g := errgroup.Group{}
	errs := martian.NewMultiError() // thread-safe error collector

	for _, routineName := range routineNames {
		g.Go(func() error {
			lastRun, err := hm.findLastRunInternal(ctx, routineName)
			if err != nil {
				errs.Add(err)
			} else {
				mu.Lock()
				results[routineName] = lastRun
				mu.Unlock()
			}

			return nil // errors are collected in errs
		})
	}

	_ = g.Wait()

	return results, errs
}

// findLastRunInternal performs the actual I/O to find the last backup for a single routine.
func (hm *HistoryManager) findLastRunInternal(
	ctx context.Context,
	routineName string,
) (*model.BackupTime, error) {
	routineStart := time.Now()
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
		slog.Duration("duration", time.Since(routineStart)),
		slog.String("lastRun", lastRun.String()))

	return lastRun, nil
}
