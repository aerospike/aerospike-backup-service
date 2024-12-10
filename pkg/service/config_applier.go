package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/aerospike"
	"github.com/reugn/go-quartz/matcher"
	"github.com/reugn/go-quartz/quartz"
)

// ConfigApplier is responsible for applying new configuration to the service.
type ConfigApplier interface {
	ApplyNewConfig(context.Context) error
}

type DefaultConfigApplier struct {
	sync.Mutex
	scheduler     quartz.Scheduler
	config        *model.Config
	backends      BackendsHolder
	clientManager aerospike.ClientManager
	handlerHolder BackupHandlerHolder
}

func NewDefaultConfigApplier(
	scheduler quartz.Scheduler,
	config *model.Config,
	backends BackendsHolder,
	manager aerospike.ClientManager,
	handlerHolder BackupHandlerHolder,
) ConfigApplier {
	return &DefaultConfigApplier{
		scheduler:     scheduler,
		config:        config,
		backends:      backends,
		clientManager: manager,
		handlerHolder: handlerHolder,
	}
}

func (a *DefaultConfigApplier) ApplyNewConfig(ctx context.Context) error {
	a.Lock()
	defer a.Unlock()

	err := a.clearPeriodicSchedulerJobs()
	if err != nil {
		return fmt.Errorf("failed to clear periodic jobs: %w", err)
	}
	a.backends.Init(a.config)

	// Refill jobs
	newHandlers := makeHandlers(ctx, a.clientManager, a.config, a.backends, a.handlerHolder)
	clear(a.handlerHolder)
	for k, v := range newHandlers {
		(a.handlerHolder)[k] = v
	}

	err = scheduleRoutines(a.scheduler, a.config, a.handlerHolder)
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
			return fmt.Errorf("cannot delete job: %w", err)
		}
	}
	return nil
}

// makeHandlers creates and returns a map of backup jobs per the configured routines.
func makeHandlers(
	ctx context.Context,
	clientManager aerospike.ClientManager,
	config *model.Config,
	backends BackendsHolder,
	oldHandlers BackupHandlerHolder,
) BackupHandlerHolder {
	handlers := make(BackupHandlerHolder)

	var wg sync.WaitGroup
	var mu sync.Mutex
	for routineName := range config.BackupRoutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler := makeHandler(ctx, clientManager, config, backends, oldHandlers, routineName)
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
	config *model.Config,
	backends BackendsHolder,
	oldHandlers BackupHandlerHolder,
	routineName string,
) *BackupRoutineHandler {
	backupService := NewBackupGo()
	backend, _ := backends.Get(routineName)

	// try to reuse lastRun from previous handler if it exists.
	var lastRun lastBackupRun
	if old, ok := oldHandlers[routineName]; ok {
		lastRun = old.lastRun
	} else {
		lastRun = backend.findLastRun(ctx) // this scan can take some time.
	}

	return newBackupRoutineHandler(config, clientManager, backupService, routineName, backend, lastRun)
}
