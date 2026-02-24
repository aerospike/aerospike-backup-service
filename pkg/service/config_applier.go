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

	invalidatedRoutines := a.config.PopInvalidatedRoutines()
	if len(invalidatedRoutines) == 0 {
		return nil
	}

	err := a.clearPeriodicSchedulerJobs(invalidatedRoutines)
	if err != nil {
		return fmt.Errorf("failed to clear periodic jobs: %w", err)
	}

	err = scheduleRoutines(
		a.scheduler,
		invalidatedRoutines,
		a.components,
		a.pathService,
	)
	if err != nil {
		return fmt.Errorf("failed to schedule periodic backups: %w", err)
	}

	routinesToSync := make([]*model.BackupRoutine, 0, len(invalidatedRoutines))
	for _, routine := range invalidatedRoutines {
		// Deleted routines are represented as disabled markers in PopInvalidatedRoutines.
		if existingRoutine, exists := a.config.Routine(routine.Name); exists {
			routinesToSync = append(routinesToSync, existingRoutine)
		}
	}

	// Scan existing backups only for routines that were invalidated and still exist.
	go a.registry.SynchroniseBackupHistory(ctx, routinesToSync)

	return nil
}

// we don't want to delete ad-hoc jobs.
func (a *DefaultConfigApplier) clearPeriodicSchedulerJobs(routines []*model.BackupRoutine) error {
	targetKeys := make(map[string]struct{}, len(routines)*2)
	for _, routine := range routines {
		routineName := routine.Name
		targetKeys[jobKey(routineName, jobTypeFull).String()] = struct{}{}
		targetKeys[jobKey(routineName, jobTypeIncremental).String()] = struct{}{}
	}

	keys, err := a.scheduler.GetJobKeys(matcher.JobGroupEquals(string(quartzGroupScheduled)))
	if err != nil {
		return fmt.Errorf("failed to fetch jobs: %w", err)
	}

	keysToDelete := make([]*quartz.JobKey, 0, len(keys))
	for _, key := range keys {
		if _, shouldDelete := targetKeys[key.String()]; !shouldDelete {
			continue
		}
		keysToDelete = append(keysToDelete, key)
	}

	slog.Info("Delete scheduled jobs", slog.Any("keys", keysToDelete))
	for _, key := range keysToDelete {
		err = a.scheduler.DeleteJob(key)
		if err != nil {
			return fmt.Errorf("failed to delete job %q: %w", key, err)
		}
	}

	return nil
}
