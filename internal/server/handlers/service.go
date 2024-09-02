package handlers

import (
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v2/internal/server/configuration"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service"
	"github.com/reugn/go-quartz/quartz"
)

type Service struct {
	config               *model.Config
	scheduler            quartz.Scheduler
	restoreManager       service.RestoreManager
	backupBackends       service.BackendsHolder
	handlerHolder        service.BackupHandlerHolder
	configurationManager configuration.Manager
	clientManger         service.ClientManager
	logger               *slog.Logger
}

func NewService(
	config *model.Config,
	scheduler quartz.Scheduler,
	restoreManager service.RestoreManager,
	backupBackends service.BackendsHolder,
	handlerHolder service.BackupHandlerHolder,
	configurationManager configuration.Manager,
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
