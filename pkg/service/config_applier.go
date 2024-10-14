package service

import (
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/reugn/go-quartz/quartz"
)

type ConfigApplier interface {
	ApplyNewConfig() error
}

type DefaultConfigApplier struct {
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
	err := a.scheduler.Clear()
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
