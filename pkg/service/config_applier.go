package service

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/quartz"
)

// ConfigApplier reschedules jobs and refreshes backup history for the routines that
// a configuration change invalidated.
type ConfigApplier interface {
	// ApplyNewConfig reschedules the jobs of every invalidated routine, removes every job of
	// the deleted ones, and queues a history sync for those that still exist. The sync runs
	// once the registry is started.
	ApplyNewConfig() error
}

// configApplier applies configuration changes by unscheduling affected cron jobs, rescheduling
// from the new config snapshot, and triggering backup-history sync for those routines.
type configApplier struct {
	mu              sync.Mutex
	backupScheduler *BackupScheduler
	registry        BackupStateRegistry
	config          *model.Config
}

var _ ConfigApplier = (*configApplier)(nil)

// NewConfigApplier returns a ConfigApplier.
func NewConfigApplier(
	backupScheduler *BackupScheduler,
	registry BackupStateRegistry,
	config *model.Config,
) ConfigApplier {
	return &configApplier{
		backupScheduler: backupScheduler,
		registry:        registry,
		config:          config,
	}
}

// ApplyNewConfig reschedules periodic jobs and rescans backup history for routines marked invalid in config.
func (a *configApplier) ApplyNewConfig() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Pop exactly once, so all follow-up steps (unschedule, reschedule, rescan)
	// operate on the same coherent invalidation snapshot.
	invalidatedRoutineNames := a.config.PopInvalidatedRoutineNames()
	if len(invalidatedRoutineNames) == 0 {
		return nil
	}

	// Quartz has no "replace these jobs atomically" API; we must do two phases:
	// 1) delete old periodic jobs for invalidated routines,
	// 2) schedule current ones.
	a.clearPeriodicSchedulerJobs(invalidatedRoutineNames)

	// Missing name means the routine was deleted after invalidation: it is unscheduled,
	// including pending ad-hoc triggers, and skipped for reschedule/rescan. Ad-hoc triggers of
	// a routine that still exists are left alone: they run even when the routine is disabled.
	routinesToApply, deletedRoutineNames := a.splitByExistence(invalidatedRoutineNames)

	a.clearAdHocSchedulerJobs(deletedRoutineNames)

	err := a.backupScheduler.ScheduleRoutines(routinesToApply)
	if err != nil {
		return fmt.Errorf("failed to schedule periodic backups: %w", err)
	}

	// Scan existing backups only for routines that were invalidated and still exist.
	a.registry.RequestHistorySync(routineNames(routinesToApply))

	return nil
}

// clearPeriodicSchedulerJobs deletes only scheduled jobs that correspond to invalidated routines.
// This keeps unaffected routines untouched. and avoids full scheduler churn.
func (a *configApplier) clearPeriodicSchedulerJobs(routineNames []string) {
	keysToDelete := make([]*quartz.JobKey, 0, len(routineNames)*2)
	for _, routineName := range routineNames {
		keysToDelete = append(keysToDelete,
			jobKey(routineName, model.BackupTypeFull),
			jobKey(routineName, model.BackupTypeIncremental))
	}

	slog.Info("Delete scheduled jobs", slog.Any("keys", keysToDelete))
	for _, key := range keysToDelete {
		_ = a.backupScheduler.DeleteJob(key) // ignore errors because we delete all jobs
	}
}

// clearAdHocSchedulerJobs deletes the pending ad-hoc jobs of the given routines. A failure is
// logged and does not stop the other routines from being rescheduled.
func (a *configApplier) clearAdHocSchedulerJobs(routineNames []string) {
	for _, routineName := range routineNames {
		if err := a.backupScheduler.DeleteAdHocJobs(routineName); err != nil {
			slog.Warn("Failed to delete ad-hoc jobs", attr.Routine(routineName), attr.Error(err))
		}
	}
}

// splitByExistence returns the configured routines among routineNames, and the names that
// no longer have a routine.
func (a *configApplier) splitByExistence(routineNames []string) ([]*model.BackupRoutine, []string) {
	existing := make([]*model.BackupRoutine, 0, len(routineNames))
	var deleted []string
	for _, routineName := range routineNames {
		actualRoutine, err := a.config.Routine(routineName)
		if err != nil {
			deleted = append(deleted, routineName)
			continue
		}

		existing = append(existing, actualRoutine)
	}

	return existing, deleted
}
