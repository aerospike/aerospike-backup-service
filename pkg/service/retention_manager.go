package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/storage"
)

type RetentionManager interface {
	deleteOldBackups(ctx context.Context) error
}

type RetentionManagerImpl struct {
	backend     BackupListReader
	storage     model.Storage
	routineName string
	policy      *model.RetentionPolicy
}

func NewBackupRetentionManager(
	backend BackupListReader,
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
	if e.policy == nil || (e.policy.FullBackups == nil && e.policy.IncrBackups == nil) {
		return nil // Retention policy is not enabled, do nothing.
	}

	fullBackups, err := e.backend.FullBackupList(ctx, model.TimeBounds{})
	if err != nil {
		return fmt.Errorf("failed to get full backups: %w", err)
	}

	timestamps := getTimestamps(fullBackups)
	if e.policy.FullBackups != nil {
		if err := e.deleteExcessFullBackups(ctx, timestamps, *e.policy.FullBackups); err != nil {
			return fmt.Errorf("failed to delete excess full backups: %w", err)
		}
	}

	if e.policy.IncrBackups != nil {
		if err := e.deleteExcessIncrementalBackups(ctx, timestamps, *e.policy.IncrBackups); err != nil {
			return fmt.Errorf("failed to delete excess incremental backups: %w", err)
		}
	}

	return nil
}

func (e *RetentionManagerImpl) deleteExcessFullBackups(
	ctx context.Context, timestamps []time.Time, retainCount int,
) error {
	if len(timestamps) <= retainCount {
		return nil
	}

	for _, t := range timestamps[:len(timestamps)-retainCount] {
		path := getTimestampPath(e.routineName, t, true)
		if err := storage.DeleteFolder(ctx, e.storage, path); err != nil {
			return fmt.Errorf("failed to delete folder at %v: %w", path, err)
		}
	}

	return nil
}

func (e *RetentionManagerImpl) deleteExcessIncrementalBackups(
	ctx context.Context, timestamps []time.Time, retainCount int,
) error {
	if len(timestamps) <= retainCount {
		return nil
	}

	if retainCount == 0 { // Delete all incremental backups.
		return storage.DeleteFolder(ctx, e.storage, getBackupRootPath(e.routineName, false))
	}

	earliestToKeep := timestamps[len(timestamps)-retainCount]
	incrBackups, err := e.backend.IncrementalBackupList(ctx, model.NewTimeBoundsTo(earliestToKeep))
	if err != nil {
		return fmt.Errorf("failed to fetch incremental backups: %w", err)
	}

	for _, b := range incrBackups {
		path := getTimestampPath(e.routineName, b.Created, false)
		if err := storage.DeleteFolder(ctx, e.storage, path); err != nil {
			return fmt.Errorf("failed to delete folder at %v: %w", path, err)
		}
	}

	return nil
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
