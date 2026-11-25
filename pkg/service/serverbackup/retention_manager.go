package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
)

// retentionManager deletes the server backups a routine's retention policy no longer keeps. It
// counts the full backups of each namespace on their own: a server backup holds one namespace,
// and the namespaces of a run need not share a creation time.
type retentionManager struct {
	catalog Catalog

	// Lock per routine. A restore holds it while the server reads the routine's backups, and a
	// configuration retrieval while it reads a backup's files.
	routineStorage *collections.LockMap
}

var _ service.BackupRetentionManager = (*retentionManager)(nil)

// NewRetentionManager returns the BackupRetentionManager of the server backup mode.
func NewRetentionManager(catalog Catalog, routineStorage *collections.LockMap) service.BackupRetentionManager {
	return &retentionManager{catalog: catalog, routineStorage: routineStorage}
}

func (m *retentionManager) ApplyRetention(ctx context.Context, routine *model.BackupRoutine) error {
	policy := routine.BackupPolicy.RetentionPolicy
	if policy.IsEmpty() {
		return nil
	}

	mu := m.routineStorage.Get(routine.Name)
	if !mu.TryLock() {
		slog.Info("Skipped retention: a restore, a configuration retrieval or another cleanup holds the routine",
			attr.Routine(routine.Name))

		return nil // The next full backup retries.
	}
	defer mu.Unlock()

	backups, err := m.catalog.AllBackups(ctx, routine.Storage, routine.Name)
	if err != nil {
		return fmt.Errorf("failed to list the backups of routine %s: %w", routine.Name, err)
	}

	var errs error
	for _, backup := range expiredBackups(policy, backups) {
		if err := m.catalog.Delete(ctx, routine.Storage, backup.Key); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to delete server backup %s: %w", backup.Key, err))
		}
	}

	return errs
}

// readLockRoutine read-locks the routine, so that retention deletes none of its backups, and
// returns the unlock. It logs when the caller has to wait for retention to finish.
func readLockRoutine(routineStorage *collections.LockMap, routineName string, logger *slog.Logger) func() {
	lock := routineStorage.Get(routineName)
	if !lock.TryRLock() {
		logger.Info("Waiting for retention of the routine to finish", attr.Routine(routineName))
		lock.RLock()
	}

	return lock.RUnlock
}

// expiredBackups returns the backups the policy no longer keeps.
func expiredBackups(policy *model.RetentionPolicy, backups []model.BackupDetails) []model.BackupDetails {
	fullTimes := make(map[string][]time.Time) // the creation times of each namespace's full backups
	for _, backup := range backups {
		if backup.BackupType() == model.BackupTypeFull {
			fullTimes[backup.Namespace] = append(fullTimes[backup.Namespace], backup.Created)
		}
	}

	// As in scan mode, full backups created at the same time count once.
	for namespace, times := range fullTimes {
		slices.SortFunc(times, time.Time.Compare)
		fullTimes[namespace] = slices.CompactFunc(times, time.Time.Equal)
	}

	return slices.DeleteFunc(slices.Clone(backups), func(backup model.BackupDetails) bool {
		return !isExpired(policy, backup, fullTimes[backup.Namespace])
	})
}

// isExpired reports whether the policy no longer keeps the backup. fullTimes holds the creation
// times of the full backups of the backup's namespace, oldest first.
func isExpired(policy *model.RetentionPolicy, backup model.BackupDetails, fullTimes []time.Time) bool {
	if backup.BackupType() == model.BackupTypeFull {
		oldest, exceeded := oldestKept(fullTimes, policy.FullBackups)

		return exceeded && backup.Created.Before(oldest)
	}

	// An incremental backup is of no use without its full backup, so by default the incremental
	// backups of the kept full backups are kept.
	count := policy.IncrBackups.Or(policy.FullBackups)
	if count.Present && count.Value == 0 {
		return true
	}

	oldest, exceeded := oldestKept(fullTimes, count)

	return exceeded && !backup.Created.After(oldest)
}

// oldestKept returns the earliest of the last count times in fullTimes, which is sorted oldest
// first. It reports false when the count keeps every full backup.
func oldestKept(fullTimes []time.Time, count optional.Optional[int]) (time.Time, bool) {
	if !count.Present || len(fullTimes) <= count.Value {
		return time.Time{}, false
	}

	return fullTimes[len(fullTimes)-count.Value], true
}
