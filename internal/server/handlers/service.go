package handlers

import (
	"context"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/configuration"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/reugn/go-quartz/quartz"
)

// Service holds all dependencies required to access business logic from endpoints.
type Service struct {
	sysCtx               context.Context //nolint:containedctx
	config               *model.Config
	configApplier        service.ConfigApplier
	scheduler            quartz.Scheduler
	restoreManager       service.RestoreManager
	configRetriever      service.ConfigRetriever
	backupReader         service.BackupReader
	registry             RunningBackupsRegistry
	configurationManager configuration.Manager
	nsValidator          aerospike.NamespaceValidator

	changeConfigLock sync.Mutex
}

func NewService(
	ctx context.Context,
	config *model.Config,
	configApplier service.ConfigApplier,
	scheduler quartz.Scheduler,
	restoreManager service.RestoreManager,
	configRetriever service.ConfigRetriever,
	backupReader service.BackupReader,
	registry RunningBackupsRegistry,
	configurationManager configuration.Manager,
	nsValidator aerospike.NamespaceValidator,
) *Service {
	return &Service{
		sysCtx:               ctx,
		config:               config,
		configApplier:        configApplier,
		scheduler:            scheduler,
		restoreManager:       restoreManager,
		configRetriever:      configRetriever,
		backupReader:         backupReader,
		registry:             registry,
		configurationManager: configurationManager,
		nsValidator:          nsValidator,
	}
}

// HTTPServerConfig returns the HTTP server config.
func (s *Service) HTTPServerConfig() *model.HTTPServerConfig {
	return s.config.ServiceConfig.GetHTTPServerOrDefault()
}

// RunningBackupsRegistry defines the interface for managing running backups and their statuses.
// this is public version of service.RunningBackupsRegistry.
type RunningBackupsRegistry interface {
	// GetRoutineState returns the current backup statistics for a routine.
	GetRoutineState(routine *model.BackupRoutine) *model.RoutineState
	// GetRunningState returns statistics for all current backups.
	GetRunningState() map[string]*model.RoutineState
	// Cancel stops all ongoing backups for a specific routine.
	Cancel(routineName string)
}
