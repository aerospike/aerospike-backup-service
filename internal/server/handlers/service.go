package handlers

import (
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v2/internal/server/configuration"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service"
	"github.com/reugn/go-quartz/quartz"
)

type Service struct {
	sync.Mutex
	config               *model.Config
	configApplier        service.ConfigApplier
	scheduler            quartz.Scheduler
	restoreManager       service.RestoreManager
	backupBackends       service.BackendsHolder
	handlerHolder        service.BackupHandlerHolder
	configurationManager configuration.Manager
	logger               *slog.Logger
}

func NewService(
	config *model.Config,
	configApplier service.ConfigApplier,
	scheduler quartz.Scheduler,
	restoreManager service.RestoreManager,
	backupBackends service.BackendsHolder,
	handlerHolder service.BackupHandlerHolder,
	configurationManager configuration.Manager,
	logger *slog.Logger,
) *Service {
	return &Service{
		config:               config,
		configApplier:        configApplier,
		scheduler:            scheduler,
		restoreManager:       restoreManager,
		backupBackends:       backupBackends,
		handlerHolder:        handlerHolder,
		configurationManager: configurationManager,
		logger:               logger,
	}
}
