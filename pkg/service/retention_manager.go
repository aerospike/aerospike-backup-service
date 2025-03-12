package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// RetentionManager defines the interface for deleting old backups.
type RetentionManager interface {
	// Run runs the retention manager. It deletes old backups based on the configured retention policy.
	deleteOldBackups(ctx context.Context) error
}

type RetentionManagerImpl struct {
	mu          sync.Mutex
	backend     BackupMetadataReader
	storage     model.Storage
	routineName string
	policy      *model.RetentionPolicy
}

func NewBackupRetentionManager(
	backend BackupMetadataReader,
	storage model.Storage,
	routineName string,
	policy *model.RetentionPolicy,
) RetentionManager {
	return &RetentionManagerImpl{
		backend:     backend,
		storage:     storage,
		routineName: routineName,
		policy:      policy,
	}
}

func (e *RetentionManagerImpl) deleteOldBackups(ctx context.Context) error {
	if !e.mu.TryLock() { // If delete operation already in progress, skip this iteration.
		return nil
	}
	defer e.mu.Unlock()

	if e.policy == nil || (e.policy.FullBackups == nil && e.policy.IncrBackups == nil) {
		slog.Info("deleteOldBackups: Retention policy disabled", "routine", e.routineName)
		return nil // Retention policy is not enabled, do nothing.
	}

	fullBackups, err := e.backend.FullBackupList(ctx, model.TimeBounds{})
	if err != nil {
		return fmt.Errorf("failed to get full backups: %w", err)
	}

	timestamps := getTimestamps(fullBackups)
	slog.Info("deleteOldBackups: got timestamps", "timestamps", timestamps, "routine", e.routineName)
	if e.policy.FullBackups != nil {
		if err := e.deleteFullBackups(ctx, timestamps, *e.policy.FullBackups); err != nil {
			return fmt.Errorf("failed to delete excess full backups: %w", err)
		}
	}

	if e.policy.IncrBackups != nil {
		if err := e.deleteIncrementalBackups(ctx, timestamps, *e.policy.IncrBackups); err != nil {
			return fmt.Errorf("failed to delete excess incremental backups: %w", err)
		}
	}

	return nil
}

func (e *RetentionManagerImpl) deleteFullBackups(
	ctx context.Context, timestamps []time.Time, retainCount int,
) error {
	if len(timestamps) <= retainCount {
		slog.Info("deleteOldBackups full: nothing to delete", "routine", e.routineName, "len", len(timestamps), "retainCount", retainCount)
		return nil
	}

	var errs error
	for _, t := range timestamps[:len(timestamps)-retainCount] {
		path := getTimestampPath(e.routineName, t, jobTypeFull)
		slog.Info("deleteOldBackups: deleting full backup", "path", path, "routine", e.routineName)
		if err := storage.DeleteFolder(ctx, e.storage, path); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to delete folder at %v: %w", path, err))
		}
	}

	return errs
}

func (e *RetentionManagerImpl) deleteIncrementalBackups(
	ctx context.Context, timestamps []time.Time, retainCount int,
) error {
	if len(timestamps) <= retainCount {
		slog.Info("deleteOldBackups incr: nothing to delete", "routine", e.routineName, "len", len(timestamps), "retainCount", retainCount)
		return nil
	}

	if retainCount == 0 { // Delete all incremental backups.
		path := getBackupRootPath(e.routineName, jobTypeIncremental)
		slog.Info("deleteOldBackups incr: deleting all incremental backups", "routine", e.routineName, "path", path)
		return storage.DeleteFolder(ctx, e.storage, path)
	}

	earliestToKeep := timestamps[len(timestamps)-retainCount]
	incrBackups, err := e.backend.IncrementalBackupList(ctx, model.NewTimeBoundsTo(earliestToKeep))
	if err != nil {
		return fmt.Errorf("failed to fetch incremental backups: %w", err)
	}

	var errs error
	for _, b := range incrBackups {
		path := getTimestampPath(e.routineName, b.Created, jobTypeIncremental)
		slog.Info("deleteOldBackups: deleting incr backup", "path", path, "routine", e.routineName)
		if err := storage.DeleteFolder(ctx, e.storage, path); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to delete folder at %v: %w", path, err))
		}
	}

	return errs
}

func getTimestamps(backups []model.BackupDetails) []time.Time {
	timeSet := make(map[time.Time]struct{}, len(backups))

	for _, obj := range backups {
		timeSet[obj.Created] = struct{}{}
	}

	times := make([]time.Time, 0, len(timeSet))
	for t := range timeSet {
		times = append(times, t)
	}

	slices.SortFunc(times, func(a, b time.Time) int {
		return a.Compare(b)
	})

	return times
}
