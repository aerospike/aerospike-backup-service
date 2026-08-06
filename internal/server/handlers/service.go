package handlers

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

// Service holds all dependencies required to access business logic from endpoints.
type Service struct {
	sysCtx          context.Context //nolint:containedctx
	configManager   ConfigManager
	backupScheduler service.AdHocScheduler
	restoreManager  service.RestoreManager
	configRetriever service.ConfigRetriever
	backupReader    service.BackupReader
	registry        RunningBackupsRegistry
}

func NewService(
	ctx context.Context,
	configManager ConfigManager,
	backupScheduler service.AdHocScheduler,
	restoreManager service.RestoreManager,
	configRetriever service.ConfigRetriever,
	backupReader service.BackupReader,
	registry RunningBackupsRegistry,
) *Service {
	return &Service{
		sysCtx:          ctx,
		configManager:   configManager,
		backupScheduler: backupScheduler,
		restoreManager:  restoreManager,
		configRetriever: configRetriever,
		backupReader:    backupReader,
		registry:        registry,
	}
}

// HTTPServerConfig returns the HTTP server config.
func (s *Service) HTTPServerConfig() *model.HTTPServerConfig {
	modelConfig, _ := s.configManager.ReadConfig(s.sysCtx).ToModel(dto.ValidationSkipTLSFiles)
	return modelConfig.ServiceConfig.GetHTTPServerOrDefault()
}

// RunningBackupsRegistry defines the interface for managing running backups and their statuses.
// this is public version of service.RunningBackupsRegistry.
type RunningBackupsRegistry interface {
	// GetRoutineState returns the current backup statistics for a routine.
	GetRoutineState(routine *model.BackupRoutine) model.RoutineState
	// GetRunningState returns statistics for all current backups.
	GetRunningState() map[string]model.RoutineState
	// Cancel stops all ongoing backups for a specific routine.
	Cancel(routineName string)
}
