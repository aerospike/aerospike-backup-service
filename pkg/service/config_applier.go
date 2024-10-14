package service

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/reugn/go-quartz/matcher"
	"github.com/reugn/go-quartz/quartz"
)

type ConfigApplier interface {
	ApplyNewConfig() error
}

type DefaultConfigApplier struct {
	sync.Mutex
	scheduler     quartz.Scheduler
	config        *model.Config
	backends      BackendsHolder
	manager       ClientManager
	handlerHolder *BackupHandlerHolder
}

func NewDefaultConfigApplier(
	scheduler quartz.Scheduler,
	config *model.Config,
	backends BackendsHolder,
	manager ClientManager,
	handlerHolder *BackupHandlerHolder,
) ConfigApplier {
	return &DefaultConfigApplier{
		scheduler:     scheduler,
		config:        config,
		backends:      backends,
		manager:       manager,
		handlerHolder: handlerHolder,
	}
}

func (a *DefaultConfigApplier) ApplyNewConfig() error {
	a.Lock()
	defer a.Unlock()

	err := a.clearPeriodicSchedulerJobs()
	if err != nil {
		return err
	}

	a.backends.SetData(BuildBackupBackends(a.config))
	clear(*a.handlerHolder)

	// Refill handlers
	newHandlers := makeHandlers(a.manager, a.config, a.backends)
	for k, v := range newHandlers {
		(*a.handlerHolder)[k] = v
	}

	err = scheduleRoutines(a.scheduler, a.config, *a.handlerHolder)
	if err != nil {
		return err
	}

	return nil
}

// we don't want to delete ad-hoc jobs
func (a *DefaultConfigApplier) clearPeriodicSchedulerJobs() error {
	keys, err := a.scheduler.GetJobKeys(matcher.JobGroupEquals(quartzGroupScheduled))
	if err != nil {
		return fmt.Errorf("cannot fetch jobs: %w", err)
	}

	slog.Info(fmt.Sprintf("Delete scheduled jobs %+v", keys))
	for _, key := range keys {
		err = a.scheduler.DeleteJob(key)
		if err != nil {
			return fmt.Errorf("cannot delete job: %w", err)
		}
	}
	return nil
}

// makeHandlers creates and returns a map of backup handlers per the configured routines.
func makeHandlers(clientManager ClientManager,
	config *model.Config,
	backends BackendsHolder,
) BackupHandlerHolder {
	handlers := make(BackupHandlerHolder)
	backupService := NewBackupGo()
	for routineName := range config.BackupRoutines {
		backend, _ := backends.Get(routineName)
		handlers[routineName] = newBackupRoutineHandler(config, clientManager, backupService, routineName, backend)
	}
	return handlers
}
