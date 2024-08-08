package handlers

import (
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"github.com/reugn/go-quartz/quartz"
)

type Service struct {
	config         *model.Config
	scheduler      quartz.Scheduler
	restoreManager service.RestoreManager
	backupBackends service.BackendsHolder
	handlerHolder  service.BackupHandlerHolder
}

func NewService(
	config *model.Config,
	scheduler quartz.Scheduler,
	restoreManager service.RestoreManager,
	backupBackends service.BackendsHolder,
	handlerHolder service.BackupHandlerHolder,

) *Service {
	return &Service{
		config:         config,
		scheduler:      scheduler,
		restoreManager: restoreManager,
		backupBackends: backupBackends,
		handlerHolder:  handlerHolder,
	}
}
