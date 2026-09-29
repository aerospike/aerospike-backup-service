package handlers

import (
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server/configuration"
	servertls "github.com/aerospike/aerospike-backup-service/v3/internal/server/tlsconfig"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
)

// Service holds all dependencies required to access business logic from endpoints.
type Service struct {
	config               *model.Config
	configApplier        service.ConfigApplier
	backupScheduler      service.AdHocScheduler
	restoreManager       service.RestoreManager
	configRetriever      service.ConfigRetriever
	backupReader         service.BackupReader
	registry             service.BackupStateRegistry
	configurationManager configuration.Manager
	nsValidator          aerospike.NamespaceValidator
	tlsProber            servertls.Prober

	changeConfigLock sync.Mutex
}

func NewService(
	config *model.Config,
	configApplier service.ConfigApplier,
	backupScheduler service.AdHocScheduler,
	restoreManager service.RestoreManager,
	configRetriever service.ConfigRetriever,
	backupReader service.BackupReader,
	registry service.BackupStateRegistry,
	configurationManager configuration.Manager,
	nsValidator aerospike.NamespaceValidator,
	tlsProber servertls.Prober,
) *Service {
	return &Service{
		config:               config,
		configApplier:        configApplier,
		backupScheduler:      backupScheduler,
		restoreManager:       restoreManager,
		configRetriever:      configRetriever,
		backupReader:         backupReader,
		registry:             registry,
		configurationManager: configurationManager,
		nsValidator:          nsValidator,
		tlsProber:            tlsProber,
	}
}
