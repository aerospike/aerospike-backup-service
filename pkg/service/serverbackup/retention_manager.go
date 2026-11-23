package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

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

	// Lock per routine. A restore holds it while the server reads the routine's backups.
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
		return nil // A restore or another cleanup holds the routine: the next full backup retries.
	}
	defer mu.Unlock()

	backups, err := m.catalog.AllBackups(ctx, routine)
	if err != nil {
		return fmt.Errorf("failed to list the backups of routine %s: %w", routine.Name, err)
	}

	var errs error
	for _, backup := range expiredBackups(policy, backups) {
		if err := m.catalog.Delete(ctx, routine, backup.Key); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to delete server backup %s: %w", backup.Key, err))
		}
	}

	return errs
}

// expiredBackups returns the backups the policy no longer keeps. It filters backups in place.
func expiredBackups(policy *model.RetentionPolicy, backups []model.BackupDetails) []model.BackupDetails {
	fulls := make(map[string][]time.Time) // the creation times of each namespace's full backups
	for _, backup := range backups {
		if backup.BackupType() == model.BackupTypeFull {
			fulls[backup.Namespace] = append(fulls[backup.Namespace], backup.Created)
		}
	}

	for _, created := range fulls {
		slices.SortFunc(created, time.Time.Compare)
	}

	return slices.DeleteFunc(backups, func(backup model.BackupDetails) bool {
		return !isExpired(policy, backup, fulls[backup.Namespace])
	})
}

// isExpired reports whether the policy no longer keeps a backup of a namespace whose full
// backups were created at fulls, oldest first.
func isExpired(policy *model.RetentionPolicy, backup model.BackupDetails, fulls []time.Time) bool {
	if backup.BackupType() == model.BackupTypeFull {
		oldest, exceeded := oldestKept(fulls, policy.FullBackups)

		return exceeded && backup.Created.Before(oldest)
	}

	// An incremental backup is of no use without its full backup, so by default the incremental
	// backups of the kept full backups are kept.
	count := policy.IncrBackups.Or(policy.FullBackups)
	if count.Present && count.Value == 0 {
		return true
	}

	oldest, exceeded := oldestKept(fulls, count)

	return exceeded && !backup.Created.After(oldest)
}

// oldestKept returns the creation time of the oldest of the last count full backups, created at
// fulls, oldest first. It reports false when the count keeps every full backup.
func oldestKept(fulls []time.Time, count optional.Optional[int]) (time.Time, bool) {
	if !count.Present || len(fulls) <= count.Value {
		return time.Time{}, false
	}

	return fulls[len(fulls)-count.Value], true
}
