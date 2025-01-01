package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
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
}

func NewDefaultConfigApplier(
	scheduler quartz.Scheduler,
	backends BackendsHolder,
	manager aerospike.ClientManager,
	handlerHolder BackupHandlerHolder,
) ConfigApplier {
	return &DefaultConfigApplier{
		scheduler:     scheduler,
		backends:      backends,
		clientManager: manager,
		handlerHolder: handlerHolder,
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

	// Refill handlers
	newHandlers := makeHandlers(ctx, a.clientManager, routines, a.backends, a.handlerHolder)
	clear(a.handlerHolder)
	for k, v := range newHandlers {
		(a.handlerHolder)[k] = v
	}

	err = scheduleRoutines(a.scheduler, routines, a.handlerHolder)
	if err != nil {
		return fmt.Errorf("failed to schedule periodic backups: %w", err)
	}

	return nil
}

// we don't want to delete ad-hoc jobs
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
	ctx context.Context,
	clientManager aerospike.ClientManager,
	routines map[string]*model.BackupRoutine,
	backends BackendsHolder,
	oldHandlers BackupHandlerHolder,
) BackupHandlerHolder {
	handlers := make(BackupHandlerHolder)

	var wg sync.WaitGroup
	var mu sync.Mutex
	for routineName, routine := range routines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler := makeHandler(ctx, clientManager, backends, oldHandlers, routineName, routine)
			mu.Lock()
			handlers[routineName] = handler
			mu.Unlock()
		}()
	}

	wg.Wait()
	return handlers
}

func makeHandler(
	ctx context.Context,
	clientManager aerospike.ClientManager,
	backends BackendsHolder,
	oldHandlers BackupHandlerHolder,
	routineName string,
	routine *model.BackupRoutine,
) *BackupRoutineHandler {
	backupService := NewBackupGo()
	backend, _ := backends.Get(routineName)

	// try to reuse lastRun from previous handler if it exists.
	var lastRun *model.LastBackupRun
	if old, ok := oldHandlers[routineName]; ok {
		lastRun = old.CurrentStat().LastRunTime
	} else {
		lastRun = backend.findLastRun(ctx) // this scan can take some time.
	}

	return newBackupRoutineHandler(clientManager, backupService, routineName, routine, backend, lastRun)
}
