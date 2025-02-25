package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/reugn/go-quartz/matcher"
	"github.com/reugn/go-quartz/quartz"
)

// ConfigApplier is responsible for applying new configuration to the service.
type ConfigApplier interface {
	ApplyNewRoutines(ctx context.Context, routines map[string]*model.BackupRoutine) error
}

type DefaultConfigApplier struct {
	mu            sync.Mutex
	scheduler     quartz.Scheduler
	backends      BackendsHolder
	clientManager aerospike.ClientManager
	handlerHolder BackupHandlerHolder
	registry      RunningBackupsRegistry
}

func NewDefaultConfigApplier(
	scheduler quartz.Scheduler,
	backends BackendsHolder,
	manager aerospike.ClientManager,
	handlerHolder BackupHandlerHolder,
	registry RunningBackupsRegistry,
) ConfigApplier {
	return &DefaultConfigApplier{
		scheduler:     scheduler,
		backends:      backends,
		clientManager: manager,
		handlerHolder: handlerHolder,
		registry:      registry,
	}
}

func (a *DefaultConfigApplier) ApplyNewRoutines(ctx context.Context, routines map[string]*model.BackupRoutine) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	err := a.clearPeriodicSchedulerJobs()
	if err != nil {
		return fmt.Errorf("failed to clear periodic jobs: %w", err)
	}
	a.backends.Init(routines)
	a.registry.StartBackupHistorySync(ctx, a.backends)

	// Refill handlers
	newHandlers := makeHandlers(a.clientManager, routines, a.backends, a.registry)
	a.handlerHolder.ReplaceContent(newHandlers)

	err = scheduleRoutines(a.scheduler, routines, a.handlerHolder)
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

// makeHandlers creates and returns a map of backup handlers per the configured routines.
func makeHandlers(
	clientManager aerospike.ClientManager,
	routines map[string]*model.BackupRoutine,
	backends BackendsHolder,
	registry RunningBackupsRegistry,
) map[string]backupRunner {
	handlers := make(map[string]backupRunner)

	backupExecutor := backupexecutor.NewDefaultBackupExecutor()
	for routineName, routine := range routines {
		backend, _ := backends.Get(routineName)
		handlers[routineName] =
			newBackupRoutineOrchestrator(clientManager, backupExecutor, routineName, routine, backend, registry)
	}

	return handlers
}
