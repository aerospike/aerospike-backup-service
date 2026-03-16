package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/quartz"
)

// ConfigApplier is responsible for applying new configuration to the service.
type ConfigApplier interface {
	// ApplyNewConfig applies new configuration to the service.
	ApplyNewConfig(ctx context.Context) error
}

type DefaultConfigApplier struct {
	mu         sync.Mutex
	scheduler  quartz.Scheduler
	registry   RunningBackupsRegistry
	components *BackupComponents
	config     *model.Config
}

func NewDefaultConfigApplier(
	scheduler quartz.Scheduler,
	registry RunningBackupsRegistry,
	components *BackupComponents,
	config *model.Config,
) ConfigApplier {
	return &DefaultConfigApplier{
		scheduler:  scheduler,
		registry:   registry,
		components: components,
		config:     config,
	}
}

func (a *DefaultConfigApplier) ApplyNewConfig(ctx context.Context) error {
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
	// Ad-hoc triggers are intentionally not touched here.
	a.clearPeriodicSchedulerJobs(invalidatedRoutineNames)

	// Missing name means the routine was deleted after invalidation:
	// it should be unscheduled only and skipped for reschedule/rescan.
	routinesToApply := a.existingRoutines(invalidatedRoutineNames)

	err := scheduleRoutines(
		a.scheduler,
		routinesToApply,
		a.components,
	)
	if err != nil {
		return fmt.Errorf("failed to schedule periodic backups: %w", err)
	}

	// Scan existing backups only for routines that were invalidated and still exist.
	go a.registry.SynchroniseBackupHistory(ctx, routinesToApply)

	return nil
}

// clearPeriodicSchedulerJobs deletes only scheduled jobs that correspond to invalidated routines.
// This keeps unaffected routines untouched. and avoids full scheduler churn.
func (a *DefaultConfigApplier) clearPeriodicSchedulerJobs(routineNames []string) {
	keysToDelete := make([]*quartz.JobKey, 0, len(routineNames)*2)
	for _, routineName := range routineNames {
		keysToDelete = append(keysToDelete,
			jobKey(routineName, jobTypeFull),
			jobKey(routineName, jobTypeIncremental))
	}

	slog.Info("Delete scheduled jobs", slog.Any("keys", keysToDelete))
	for _, key := range keysToDelete {
		_ = a.scheduler.DeleteJob(key) // ignore errors because we delete all jobs
		jobStore.Remove(key.String())
	}
}

func (a *DefaultConfigApplier) existingRoutines(routineNames []string) []*model.BackupRoutine {
	existing := make([]*model.BackupRoutine, 0, len(routineNames))
	for _, routineName := range routineNames {
		if actualRoutine, ok := a.config.Routine(routineName); ok {
			existing = append(existing, actualRoutine)
		}
	}

	return existing
}
