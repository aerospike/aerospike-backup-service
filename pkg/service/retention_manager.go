package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
)

// RetentionManager defines the interface for deleting old backups.
type RetentionManager interface {
	// Run runs the retention manager. It deletes old backups based on the configured retention policy.
	deleteOldBackups(ctx context.Context, routine *model.BackupRoutine) error
}

type RetentionManagerImpl struct {
	backendService BackupReaderWriter

	// Lock per routine. The restore service reads backup data,
	// while the retention manager deletes backup data.
	// Locks are required to avoid race conditions.
	routineStorage *collections.LockMap
}

var _ RetentionManager = (*RetentionManagerImpl)(nil)

func NewBackupRetentionManager(
	backendService BackupReaderWriter,
	routineStorage *collections.LockMap,
) *RetentionManagerImpl {
	return &RetentionManagerImpl{
		backendService: backendService,
		routineStorage: routineStorage,
	}
}

func (e *RetentionManagerImpl) deleteOldBackups(ctx context.Context, routine *model.BackupRoutine) error {
	policy := routine.BackupPolicy.RetentionPolicy
	if policy.IsEmpty() {
		return nil // Retention policy is not enabled, do nothing.
	}

	mu := e.routineStorage.Get(routine.Name)
	if !mu.TryLock() { // retention uses Lock to exclude restores while deleting.
		return nil // If delete or restore operation already in progress, skip this iteration.
	}
	defer mu.Unlock()

	fullBackups, err := e.backendService.GetBackups(ctx, NewFullBackupFilter(routine))
	if err != nil {
		return fmt.Errorf("failed to get full backups: %w", err)
	}

	timestamps := getTimestamps(fullBackups)
	if policy.FullBackups.Present {
		if err := e.deleteFullBackups(ctx, timestamps, policy.FullBackups.Value, routine, fullBackups); err != nil {
			return fmt.Errorf("failed to delete excess full backups: %w", err)
		}
	}

	// Incremental backups cannot exist without their corresponding full backup.
	// If retention policy is not set for incremental (meaning keep all incrementals),
	// delete them based on full backups.
	effectiveIncrementalRetention := policy.IncrBackups.Or(policy.FullBackups)
	if effectiveIncrementalRetention.Present {
		if err := e.deleteIncrementalBackups(ctx, timestamps, effectiveIncrementalRetention.Value, routine); err != nil {
			return fmt.Errorf("failed to delete excess incremental backups: %w", err)
		}
	}

	return nil
}

func (e *RetentionManagerImpl) deleteFullBackups(
	ctx context.Context,
	timestamps []time.Time,
	retainCount int,
	routine *model.BackupRoutine,
	backups []model.BackupDetails,
) error {
	if len(timestamps) <= retainCount {
		return nil
	}

	earliest := timestamps[len(timestamps)-retainCount]
	var errs error
	for _, b := range backups {
		if !b.Created.Before(earliest) {
			continue
		}
		if err := e.backendService.Delete(ctx, routine, extractBackupDirFromKey(b.Key)); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to delete folder at %v: %w", b.Key, err))
		}
	}

	return errs
}

func (e *RetentionManagerImpl) deleteIncrementalBackups(
	ctx context.Context, timestamps []time.Time, retainCount int, routine *model.BackupRoutine,
) error {
	if retainCount == 0 { // Delete all incremental backups.
		path := backupRootPath(routine.Name, jobTypeIncremental)
		return e.backendService.Delete(ctx, routine, path)
	}

	if len(timestamps) <= retainCount {
		return nil
	}

	earliest := timestamps[len(timestamps)-retainCount]
	incrBackups, err := e.backendService.GetBackups(ctx, NewIncrementalBackupFilter(routine).WithToTime(earliest))
	if err != nil {
		return fmt.Errorf("failed to fetch incremental backups: %w", err)
	}

	var errs error
	for _, b := range incrBackups {
		if err := e.backendService.Delete(ctx, routine, extractBackupDirFromKey(b.Key)); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to delete folder at %v: %w", b.Key, err))
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
