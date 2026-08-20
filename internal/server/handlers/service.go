package handlers

import (
	"context"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
)

// Service holds all dependencies required to access business logic from endpoints.
type Service struct {
	sysCtx               context.Context //nolint:containedctx
	config               *model.Config
	configApplier        configApplier
	backupScheduler      service.AdHocScheduler
	restoreManager       service.RestoreManager
	configRetriever      configRetriever
	backupReader         service.BackupReader
	registry             runningBackupsRegistry
	configurationManager manager
	nsValidator          aerospike.NamespaceValidator

	changeConfigLock sync.Mutex
}

func NewService(
	ctx context.Context,
	config *model.Config,
	configApplier configApplier,
	backupScheduler service.AdHocScheduler,
	restoreManager service.RestoreManager,
	configRetriever configRetriever,
	backupReader service.BackupReader,
	registry runningBackupsRegistry,
	configurationManager manager,
	nsValidator aerospike.NamespaceValidator,
) *Service {
	return &Service{
		sysCtx:               ctx,
		config:               config,
		configApplier:        configApplier,
		backupScheduler:      backupScheduler,
		restoreManager:       restoreManager,
		configRetriever:      configRetriever,
		backupReader:         backupReader,
		registry:             registry,
		configurationManager: configurationManager,
		nsValidator:          nsValidator,
	}
}

// runningBackupsRegistry defines the interface for managing running backups and their statuses.
// this is public version of service.RunningBackupsRegistry.
type runningBackupsRegistry interface {
	// GetRoutineState returns the current backup statistics for a routine.
	GetRoutineState(routine *model.BackupRoutine) model.RoutineState
	// GetRunningState returns statistics for all current backups.
	GetRunningState() map[string]model.RoutineState
	// Cancel stops all ongoing backups for a specific routine.
	Cancel(routineName string)
}

// manager reads and writes service configuration.
type manager interface {
	// Read reads the configuration from the source.
	Read(ctx context.Context) (*model.Config, error)
	// Write writes the configuration to the source.
	Write(ctx context.Context, config *model.Config) error
}

// configApplier applies new configuration to the service.
type configApplier interface {
	// ApplyNewConfig applies new configuration to the service.
	ApplyNewConfig(ctx context.Context) error
}

// configRetriever reads backed-up Aerospike configuration from storage.
type configRetriever interface {
	// RetrieveConfiguration returns backed up Aerospike configuration.
	RetrieveConfiguration(ctx context.Context, routine *model.BackupRoutine, t time.Time) ([]byte, error)
}
