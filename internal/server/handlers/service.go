package handlers

import (
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/configuration"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/reugn/go-quartz/quartz"
)

type Service struct {
	config               *model.Config
	configApplier        service.ConfigApplier
	scheduler            quartz.Scheduler
	restoreManager       service.RestoreManager
	backupBackends       service.BackendsHolder
	handlerHolder        service.BackupHandlerHolder
	configurationManager configuration.Manager
	logger               *slog.Logger
	nsValidator          aerospike.NamespaceValidator
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
	nsValidator aerospike.NamespaceValidator,
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
		nsValidator:          nsValidator,
	}
}
