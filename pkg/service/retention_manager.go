package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
)

// RetentionManager defines the interface for deleting old backups.
type RetentionManager interface {
	// Run runs the retention manager. It deletes old backups based on the configured retention policy.
	deleteOldBackups(ctx context.Context, routineName string) error
}

type RetentionManagerImpl struct {
	backendService BackupReaderWriter
	config         *model.Config

	// Lock per routine. The restore service reads backup data,
	// while the retention manager deletes backup data.
	// Locks are required to avoid race conditions.
	routineStorage *util.LockMap
}

var _ RetentionManager = (*RetentionManagerImpl)(nil)

func NewBackupRetentionManager(
	backendService BackupReaderWriter,
	config *model.Config,
	routineStorage *util.LockMap,
) *RetentionManagerImpl {
	return &RetentionManagerImpl{
		backendService: backendService,
		config:         config,
		routineStorage: routineStorage,
	}
}

func (e *RetentionManagerImpl) deleteOldBackups(ctx context.Context, routineName string) error {
	routine, found := e.config.Routine(routineName)
	if !found {
		return fmt.Errorf("routine '%s' does not exist", routineName)
	}

	policy := routine.BackupPolicy.RetentionPolicy
	if policy == nil || (policy.FullBackups == nil && policy.IncrBackups == nil) {
		return nil // Retention policy is not enabled, do nothing.
	}

	mu := e.routineStorage.Get(routineName)
	if !mu.TryLock() { // If delete or restore operation already in progress, skip this iteration.
		return nil
	}
	defer mu.Unlock()

	fullBackups, err := e.backendService.GetBackups(ctx, NewFullBackupFilter(routineName))
	if err != nil {
		return fmt.Errorf("failed to get full backups: %w", err)
	}

	timestamps := getTimestamps(fullBackups)
	if policy.FullBackups != nil {
		if err := e.deleteFullBackups(ctx, timestamps, *policy.FullBackups, routineName); err != nil {
			return fmt.Errorf("failed to delete excess full backups: %w", err)
		}
	}

	effectiveIncrementalRetention := policy.GetIncrementalRetentionCount()
	if effectiveIncrementalRetention != nil {
		if err := e.deleteIncrementalBackups(ctx, timestamps, *effectiveIncrementalRetention, routineName); err != nil {
			return fmt.Errorf("failed to delete excess incremental backups: %w", err)
		}
	}

	return nil
}

func (e *RetentionManagerImpl) deleteFullBackups(
	ctx context.Context, timestamps []time.Time, retainCount int, routineName string,
) error {
	if len(timestamps) <= retainCount {
		return nil
	}

	var errs error
	for _, t := range timestamps[:len(timestamps)-retainCount] {
		path := getTimestampPath(routineName, t, jobTypeFull)
		if err := e.backendService.Delete(ctx, routineName, path); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to delete folder at %v: %w", path, err))
		}
	}

	return errs
}

func (e *RetentionManagerImpl) deleteIncrementalBackups(
	ctx context.Context, timestamps []time.Time, retainCount int, routineName string,
) error {
	if retainCount == 0 { // Delete all incremental backups.
		path := getBackupRootPath(routineName, jobTypeIncremental)
		return e.backendService.Delete(ctx, routineName, path)
	}

	if len(timestamps) <= retainCount {
		return nil
	}

	earliest := timestamps[len(timestamps)-retainCount]
	incrBackups, err := e.backendService.GetBackups(ctx, NewIncrementalBackupFilter(routineName).WithToTime(earliest))
	if err != nil {
		return fmt.Errorf("failed to fetch incremental backups: %w", err)
	}

	var errs error
	for _, b := range incrBackups {
		path := getTimestampPath(routineName, b.Created, jobTypeIncremental)
		if err := e.backendService.Delete(ctx, routineName, path); err != nil {
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
