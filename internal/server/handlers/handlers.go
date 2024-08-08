package handlers

import (
	"log/slog"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"github.com/reugn/go-quartz/quartz"
)

type Service struct {
	config               *model.Config
	scheduler            quartz.Scheduler
	restoreManager       service.RestoreManager
	backupBackends       service.BackendsHolder
	handlerHolder        service.BackupHandlerHolder
	configurationManager service.ConfigurationManager
	logger               *slog.Logger
}

func NewService(
	config *model.Config,
	scheduler quartz.Scheduler,
	restoreManager service.RestoreManager,
	backupBackends service.BackendsHolder,
	handlerHolder service.BackupHandlerHolder,
	configurationManager service.ConfigurationManager,
	logger *slog.Logger,
) *Service {
	return &Service{
		config:               config,
		scheduler:            scheduler,
		restoreManager:       restoreManager,
		backupBackends:       backupBackends,
		handlerHolder:        handlerHolder,
		configurationManager: configurationManager,
		logger:               logger,
	}
}
