package handlers

import (
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/pkg/service"
	"github.com/reugn/go-quartz/quartz"
)

type Service struct {
	config               *dto.Config
	scheduler            quartz.Scheduler
	restoreManager       service.RestoreManager
	backupBackends       service.BackendsHolder
	handlerHolder        service.BackupHandlerHolder
	configurationManager service.ConfigurationManager
	clientManger         service.ClientManager
	logger               *slog.Logger
}

func NewService(
	config *dto.Config,
	scheduler quartz.Scheduler,
	restoreManager service.RestoreManager,
	backupBackends service.BackendsHolder,
	handlerHolder service.BackupHandlerHolder,
	configurationManager service.ConfigurationManager,
	clientManger service.ClientManager,
	logger *slog.Logger,
) *Service {
	return &Service{
		config:               config,
		scheduler:            scheduler,
		restoreManager:       restoreManager,
		backupBackends:       backupBackends,
		handlerHolder:        handlerHolder,
		configurationManager: configurationManager,
		clientManger:         clientManger,
		logger:               logger,
	}
}
