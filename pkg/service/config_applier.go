package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/matcher"
	"github.com/reugn/go-quartz/quartz"
)

// ConfigApplier is responsible for applying new configuration to the service.
type ConfigApplier interface {
	// ApplyNewConfig applies new configuration to the service.
	ApplyNewConfig(ctx context.Context) error
}

type DefaultConfigApplier struct {
	mu          sync.Mutex
	scheduler   quartz.Scheduler
	registry    RunningBackupsRegistry
	components  *BackupComponents
	config      *model.Config
	pathService PathService
}

func NewDefaultConfigApplier(
	scheduler quartz.Scheduler,
	registry RunningBackupsRegistry,
	components *BackupComponents,
	config *model.Config,
	pathService PathService,
) ConfigApplier {
	return &DefaultConfigApplier{
		scheduler:   scheduler,
		registry:    registry,
		components:  components,
		config:      config,
		pathService: pathService,
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

	err := a.clearPeriodicSchedulerJobs(invalidatedRoutineNames)
	if err != nil {
		return fmt.Errorf("failed to clear periodic jobs: %w", err)
	}

	routinesToApply := a.existingRoutines(invalidatedRoutineNames)

	err = scheduleRoutines(
		a.scheduler,
		routinesToApply,
		a.components,
		a.pathService,
	)
	if err != nil {
		return fmt.Errorf("failed to schedule periodic backups: %w", err)
	}

	// Scan existing backups only for routines that were invalidated and still exist.
	go a.registry.SynchroniseBackupHistory(ctx, routinesToApply)

	return nil
}

func (a *DefaultConfigApplier) clearPeriodicSchedulerJobs(routineNames []string) error {
	// Delete only scheduled jobs that correspond to invalidated routines.
	// This keeps unaffected routines untouched and avoids full scheduler churn.
	keys, err := a.scheduler.GetJobKeys(matcher.JobGroupEquals(string(quartzGroupScheduled)))
	if err != nil {
		return fmt.Errorf("failed to fetch jobs: %w", err)
	}

	existing := make(map[string]*quartz.JobKey, len(keys))
	for _, key := range keys {
		existing[key.String()] = key
	}

	keysToDelete := make([]*quartz.JobKey, 0, len(routineNames)*2)
	deletedKeyStrings := make([]string, 0, len(routineNames)*2)
	for _, routineName := range routineNames {
		for _, keyString := range scheduledJobKeyStrings(routineName) {
			if key, ok := existing[keyString]; ok {
				keysToDelete = append(keysToDelete, key)
				deletedKeyStrings = append(deletedKeyStrings, keyString)
			}
		}
	}

	slog.Info("Delete scheduled jobs", slog.Any("keys", keysToDelete))
	for _, key := range keysToDelete {
		err = a.scheduler.DeleteJob(key)
		if err != nil {
			return fmt.Errorf("failed to delete job %q: %w", key, err)
		}
	}

	// Keep ad-hoc job source in sync with scheduler state.
	// IMPORTANT: remove from jobStore only after scheduler deletions succeed.
	// This preserves consistency if DeleteJob fails midway.
	for _, keyString := range deletedKeyStrings {
		jobStore.Remove(keyString)
	}

	return nil
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

func scheduledJobKeyStrings(routineName string) []string {
	return []string{
		jobKey(routineName, jobTypeFull).String(),
		jobKey(routineName, jobTypeIncremental).String(),
	}
}
