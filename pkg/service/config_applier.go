package service

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/matcher"
	"github.com/reugn/go-quartz/quartz"
)

// ConfigApplier is responsible for applying new configuration to the service.
type ConfigApplier interface {
	ApplyNewRoutines(routines map[string]*model.BackupRoutine) error
}

type DefaultConfigApplier struct {
	mu         sync.Mutex
	scheduler  quartz.Scheduler
	registry   RunningBackupsRegistry
	components *BackupComponents
}

func NewDefaultConfigApplier(
	scheduler quartz.Scheduler,
	registry RunningBackupsRegistry,
	components *BackupComponents,
) ConfigApplier {
	return &DefaultConfigApplier{
		scheduler:  scheduler,
		registry:   registry,
		components: components,
	}
}

func (a *DefaultConfigApplier) ApplyNewRoutines(routines map[string]*model.BackupRoutine) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	err := a.clearPeriodicSchedulerJobs()
	if err != nil {
		return fmt.Errorf("failed to clear periodic jobs: %w", err)
	}

	// Scan existing backups to find the last successful runs for every routine.
	go a.registry.SynchroniseBackupHistory()

	err = scheduleRoutines(a.scheduler, routines, a.components)
	if err != nil {
		return fmt.Errorf("failed to schedule periodic backups: %w", err)
	}

	return nil
}

// we don't want to delete ad-hoc jobs.
func (a *DefaultConfigApplier) clearPeriodicSchedulerJobs() error {
	keys, err := a.scheduler.GetJobKeys(matcher.JobGroupEquals(string(quartzGroupScheduled)))
	if err != nil {
		return fmt.Errorf("cannot fetch jobs: %w", err)
	}

	slog.Info("Delete scheduled jobs", slog.Any("keys", keys))
	for _, key := range keys {
		err = a.scheduler.DeleteJob(key)
		if err != nil {
			return fmt.Errorf("cannot delete job %q: %w", key, err)
		}
	}

	return nil
}
