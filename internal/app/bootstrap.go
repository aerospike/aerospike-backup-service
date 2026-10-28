package app

import (
	"context"
	"fmt"
	"log/slog"

	backup "github.com/aerospike/aerospike-backup-service/v3"
	"github.com/aerospike/aerospike-backup-service/v3/internal/log"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/configuration"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	servertls "github.com/aerospike/aerospike-backup-service/v3/internal/server/tlsconfig"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/prometheus"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/serverbackup"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	u "github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/reugn/go-quartz/quartz"
)

// Components group the long-running parts of the service.
// Every field is always a non-nil interface after InitComponents succeeds. A component that
// has nothing to do is still constructed as a no-op rather than left nil, so callers never nil-check.
type Components struct {
	Scheduler        quartz.Scheduler
	Servers          []server.HTTP
	MetricsCollector *prometheus.MetricsCollector
	TLSProvider      servertls.TLSProvider

	registry      service.BackupStateRegistry
	restoreJobs   component
	configApplier service.ConfigApplier
}

// component is anything whose work outlives a single request: it takes the run context
// from Components.Start and ends when that context is canceled.
// Start should be called exactly once, by Components.Start.
type component interface {
	Start(ctx context.Context)
}

// Run starts every component and serves until ctx is canceled or a listener stops. It
// blocks for as long as the service runs: a caller that wants the service up without
// giving up its goroutine - a test serving the handler itself - uses Start instead.
func (c *Components) Run(ctx context.Context) error {
	if err := c.Start(ctx); err != nil {
		return err
	}

	return server.Run(ctx, c.Servers)
}

// Start brings up every background component, applies the loaded configuration to the
// running scheduler, and returns. It is the one place the run context is handed out: a
// component that outlives a single request takes its lifetime from here, never from the
// context that built the graph. It is called exactly once.
func (c *Components) Start(ctx context.Context) error {
	c.registry.Start(ctx)
	c.restoreJobs.Start(ctx)
	c.Scheduler.Start(ctx)
	c.MetricsCollector.Start(ctx, prometheus.CollectInterval)
	c.TLSProvider.Start(ctx)

	if err := c.configApplier.ApplyNewConfig(ctx); err != nil {
		return fmt.Errorf("failed to apply configuration: %w", err)
	}

	return nil
}

// InitComponents builds the full object graph.
// Components are created and wired but not started: no goroutines, listeners, or
// watchers run here. The caller decides when to Start them, normally through Run.
//
// ctx is the build context: it is for loading only. Nothing may capture it or launch
// work on it - a component that outlives a single request takes its lifetime from Start.
//
// Collaborators (other components) are non-nil interfaces. If a collaborator is unused
// or a feature is disabled, pass a no-op implementation, never nil, and do not add
// nil-receiver guards.
//
// Configuration and other data values stay as values. Do not wrap those in interfaces.
//
//nolint:funlen // deliberately keep all initialization in a single function.
func InitComponents(
	ctx context.Context,
	configFile string,
	remote bool,
) (*Components, error) {
	resolver := secrets.NewResolver()
	tlsProber := servertls.NewProber(secrets.NewClusterTLSResolver(resolver))
	s3Accessor := storage.NewS3StorageAccessor(resolver)
	operations := newStorageOperations(resolver, s3Accessor)
	clientManager, nsValidator := newAerospikeLayer(resolver)

	config, configurationManager, err := configuration.Load(ctx, configFile, remote, operations, tlsProber)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	appLogger, err := initLogger(config)
	if err != nil {
		return nil, err
	}

	scheduler, err := service.NewScheduler(appLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	pathService := service.NewPathService(config.ServiceConfig.GetBackupCommonOrDefault().TimestampFormat)
	var routineStorage u.LockMap

	// components that are specific to mode - server/scan backup
	components := newComponentsForMode(config, pathService, operations, s3Accessor, clientManager, &routineStorage)

	completionHandler := service.NewBackupCompletionHandler(
		components.registry,
		components.retentionManager,
		components.clusterConfigWriter,
	)
	backupOrchestrator := service.NewBackupOrchestrator(
		components.registry,
		completionHandler,
		service.NewBackupReporter(),
		components.startController,
		components.routineRunner,
	)
	backupScheduler := service.NewBackupScheduler(scheduler, backupOrchestrator)
	configApplier := service.NewConfigApplier(backupScheduler, components.registry, nsValidator, config)

	restoreJobs := service.NewRestoreJobsHolder()
	restoreValidator := service.NewRestoreValidator(components.startController, config)

	restoreMgr := service.NewRestoreManager(
		components.restoreExecutor,
		clientManager,
		restoreJobs,
		components.backupReader,
		&routineStorage,
		restoreValidator,
	)

	metricsCollector := prometheus.NewMetricsCollector(components.registry.GetRunningState, restoreJobs.StatusCounts)

	configRetriever := service.NewConfigRetriever(components.backupReader, pathService, operations)
	srv := handlers.NewService(
		config,
		configApplier,
		backupScheduler,
		restoreMgr,
		configRetriever,
		components.backupReader,
		components.registry,
		configurationManager,
		tlsProber,
	)

	var servers []server.HTTP
	configHTTP := config.ServiceConfig.GetServerHTTPOrDefault()
	if !configHTTP.Disabled {
		servers = append(servers, server.NewServerHTTP(configHTTP, srv))
	}

	tlsProvider := servertls.NoReload()
	configHTTPS := config.ServiceConfig.GetServerHTTPSOrDefault()
	if !configHTTPS.Disabled {
		httpsServer, reloader, err := newServerHTTPS(ctx, configHTTPS, srv, resolver)
		if err != nil {
			return nil, err
		}
		tlsProvider = reloader
		servers = append(servers, httpsServer)
	}

	return &Components{
		Scheduler:        scheduler,
		Servers:          servers,
		MetricsCollector: metricsCollector,
		TLSProvider:      tlsProvider,
		registry:         components.registry,
		restoreJobs:      restoreJobs,
		configApplier:    configApplier,
	}, nil
}

func newServerHTTPS(
	ctx context.Context,
	configHTTPS *model.ServerConfigHTTPS,
	srv *handlers.Service,
	resolver secrets.Resolver,
) (server.HTTP, servertls.TLSProvider, error) {
	tlsProvider := servertls.NewCertificateProvider(configHTTPS, resolver)
	if err := tlsProvider.Load(ctx); err != nil { // initial TLS material load.
		return nil, nil, fmt.Errorf("failed to create HTTPS server: %w", err)
	}

	tlsConfig, err := servertls.NewTLSConfig(configHTTPS, tlsProvider)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create HTTPS server: %w", err)
	}

	return server.NewServerHTTPS(configHTTPS, srv, tlsConfig), tlsProvider, nil
}

func newStorageOperations(resolver secrets.Resolver, s3Accessor storage.Accessor) storage.Operations {
	return storage.NewOperations(
		s3Accessor,
		storage.NewGcpStorageAccessor(resolver),
		storage.NewAzureStorageAccessor(resolver),
		storage.NewLocalStorageAccessor(),
	)
}

// backupMode is the part of the object graph that depends on the configured backup mode.
// Each mode wires its own implementations behind the same interfaces, so no component
// checks the mode itself.
type backupMode struct {
	backupReader        service.BackupReader
	registry            service.BackupStateRegistry
	startController     service.StartController
	retentionManager    service.BackupRetentionManager
	clusterConfigWriter service.ClusterConfigWriter
	routineRunner       service.RoutineBackupRunner
	restoreExecutor     restoreexecutor.Restore
}

func newComponentsForMode(
	config *model.Config,
	pathService service.PathService,
	operations storage.Operations,
	s3Clients serverbackup.S3ClientProvider,
	clientManager aerospike.ClientManager,
	routineStorage *u.LockMap,
) backupMode {
	if config.ServiceConfig.GetBackupCommonOrDefault().GetBackupModeOrDefault() == model.BackupModeServer {
		return newServerBackupMode(config, s3Clients, clientManager)
	}

	return newScanBackupMode(config, pathService, operations, clientManager, routineStorage)
}

// newScanBackupMode wires backups that the service scans from the cluster and writes itself.
func newScanBackupMode(
	config *model.Config,
	pathService service.PathService,
	operations storage.Operations,
	clientManager aerospike.ClientManager,
	routineStorage *u.LockMap,
) backupMode {
	catalog := service.NewBackupCatalog(pathService, operations)
	registry := service.NewBackupStateRegistry(service.NewHistoryManager(catalog), config)
	namespaceRunner := service.NewNamespaceBackupRunner(
		backupexecutor.NewBackupExecutor(clientManager, operations),
		catalog,
		pathService,
	)

	return backupMode{
		backupReader:     catalog,
		registry:         registry,
		startController:  service.NewStartController(registry, service.NewStartDecider()),
		retentionManager: service.NewBackupRetentionManager(catalog, routineStorage),
		clusterConfigWriter: service.NewClusterConfigWriter(
			pathService,
			operations,
			aerospike.NewClusterConfigSource(clientManager),
		),
		routineRunner:   service.NewRoutineBackupRunner(namespaceRunner, aerospike.NewNamespaceResolver(clientManager)),
		restoreExecutor: restoreexecutor.NewRestoreExecutor(operations),
	}
}

// newServerBackupMode wires backups that the Aerospike server writes to S3 itself.
// The server owns the backup layout, so the service neither applies retention nor
// stores the cluster configuration next to a backup.
func newServerBackupMode(
	config *model.Config,
	s3Clients serverbackup.S3ClientProvider,
	clientManager aerospike.ClientManager,
) backupMode {
	catalog := serverbackup.NewCatalog(s3Clients)

	return backupMode{
		backupReader:        catalog,
		registry:            service.NewBackupStateRegistry(service.NewHistoryManager(catalog), config),
		startController:     serverbackup.NewStartController(),
		retentionManager:    service.NopRetentionManager(),
		clusterConfigWriter: service.NopClusterConfigWriter(),
		routineRunner: serverbackup.NewRoutineRunner(
			clientManager,
			aerospike.NewNamespaceResolver(clientManager),
			catalog,
		),
		restoreExecutor: serverbackup.NewRestoreExecutor(catalog),
	}
}

func newAerospikeLayer(resolver secrets.Resolver) (aerospike.ClientManager, aerospike.NamespaceValidator) {
	passwordResolver := secrets.NewPasswordResolver(resolver)
	clientManager := aerospike.NewClientManager(
		aerospike.NewClientFactory(passwordResolver, secrets.NewClusterTLSResolver(resolver)),
		aerospike.DefaultCloseDelay,
	)
	return clientManager, aerospike.NewNamespaceValidator(clientManager)
}

func initLogger(config *model.Config) (*slog.Logger, error) {
	logHandler, err := log.NewHandler(config.ServiceConfig.GetLoggerOrDefault())
	if err != nil {
		return nil, fmt.Errorf("failed to configure logging: %w", err)
	}
	logger := slog.New(logHandler)
	slog.SetDefault(logger)
	configStr, _ := decoder.Marshal(dto.NewConfigFromModel(config), decoder.JSON, true)
	slog.Info("Aerospike Backup Service",
		slog.String("version", backup.Version),
		slog.String("commit", backup.CommitHash),
		slog.String("buildTime", backup.BuildTime),
		slog.String("config", string(configStr)))

	return logger, nil
}
