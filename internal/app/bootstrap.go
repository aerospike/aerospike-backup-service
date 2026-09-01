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
	CertReloader     servertls.Reloader
}

// InitComponents builds the full object graph.
// Components are created and wired but not started: no goroutines, listeners, or
// watchers run here. The caller decides when to Start/Stop them.
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
	tlsProber := servertls.NewProber(resolver)
	operations := newStorageOperations(resolver)
	clientManager, nsValidator := newAerospikeLayer(resolver)

	config, configurationManager, err := configuration.Load(ctx, configFile, remote, nsValidator, operations, tlsProber)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	appLogger := initLogger(config)

	scheduler, err := service.NewScheduler(ctx, appLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	pathService := service.NewPathService(config.ServiceConfig.GetBackupCommonOrDefault().TimestampFormat)
	catalog := service.NewBackupCatalog(pathService, operations)
	history := service.NewHistoryManager(catalog)
	registry := service.NewBackupStateRegistry(history, config)
	var routineStorage u.LockMap
	retentionManager := service.NewBackupRetentionManager(catalog, &routineStorage)
	clusterConfigWriter := service.NewClusterConfigWriter(
		pathService,
		operations,
		aerospike.NewClusterConfigSource(clientManager),
	)
	completionHandler := service.NewBackupCompletionHandler(registry, retentionManager, clusterConfigWriter)
	backupExecutor := backupexecutor.NewBackupExecutor(clientManager, operations)
	startController := service.NewStartController(registry, service.NewStartDecider())
	namespaceRunner := service.NewNamespaceBackupRunner(backupExecutor, catalog, pathService)
	namespaceResolver := aerospike.NewNamespaceResolver(clientManager)
	routineBackupRunner := service.NewRoutineBackupRunner(
		namespaceRunner,
		namespaceResolver,
	)

	backupOrchestrator := service.NewBackupOrchestrator(
		registry,
		completionHandler,
		service.NewBackupReporter(),
		startController,
		routineBackupRunner,
	)
	backupScheduler := service.NewBackupScheduler(scheduler, backupOrchestrator)
	configApplier := service.NewConfigApplier(backupScheduler, registry, config)

	err = configApplier.ApplyNewConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to apply new config: %w", err)
	}

	restoreJobs := service.NewRestoreJobsHolder()
	restoreValidator := service.NewRestoreValidator(startController, config)

	restoreMgr := service.NewRestoreManager(
		restoreexecutor.NewRestoreExecutor(operations),
		clientManager,
		restoreJobs,
		catalog,
		&routineStorage,
		restoreValidator,
	)

	metricsCollector := prometheus.NewMetricsCollector(registry.GetRunningState, restoreJobs.StatusCounts)

	configRetriever := service.NewConfigRetriever(catalog, pathService, operations)
	srv := handlers.NewService(
		ctx,
		config,
		configApplier,
		backupScheduler,
		restoreMgr,
		configRetriever,
		catalog,
		registry,
		configurationManager,
		nsValidator,
		tlsProber,
	)

	var servers []server.HTTP
	configHTTP := config.ServiceConfig.GetServerHTTPOrDefault()
	if !configHTTP.Disabled {
		servers = append(servers, server.NewServerHTTP(ctx, configHTTP, srv))
	}

	certReloader := servertls.NoReload()
	configHTTPS := config.ServiceConfig.GetServerHTTPSOrDefault()
	if !configHTTPS.Disabled {
		httpsServer, reloader, err := newServerHTTPS(ctx, configHTTPS, srv, resolver)
		if err != nil {
			return nil, err
		}
		certReloader = reloader
		servers = append(servers, httpsServer)
	}

	return &Components{
		Scheduler:        scheduler,
		Servers:          servers,
		MetricsCollector: metricsCollector,
		CertReloader:     certReloader,
	}, nil
}

func newServerHTTPS(
	ctx context.Context,
	configHTTPS *model.ServerConfigHTTPS,
	srv *handlers.Service,
	resolver secrets.Resolver,
) (server.HTTP, servertls.Reloader, error) {
	reloader := servertls.NewCertificateReloader(
		configHTTPS, resolver, servertls.DefaultWatchInterval,
	)
	if err := reloader.Load(ctx); err != nil { // initial TLS material load.
		return nil, nil, fmt.Errorf("failed to create HTTPS server: %w", err)
	}

	tlsConfig, err := servertls.NewTLSConfig(configHTTPS, reloader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create HTTPS server: %w", err)
	}

	return server.NewServerHTTPS(ctx, configHTTPS, srv, tlsConfig), reloader, nil
}

func newStorageOperations(resolver secrets.Resolver) storage.Operations {
	return storage.NewOperations(
		storage.NewS3StorageAccessor(resolver),
		storage.NewGcpStorageAccessor(resolver),
		storage.NewAzureStorageAccessor(resolver),
		storage.NewLocalStorageAccessor(),
	)
}

func newAerospikeLayer(resolver secrets.Resolver) (aerospike.ClientManager, aerospike.NamespaceValidator) {
	passwordResolver := secrets.NewPasswordResolver(resolver)
	clientManager := aerospike.NewClientManager(
		aerospike.NewClientFactory(passwordResolver),
		aerospike.DefaultCloseDelay,
	)
	return clientManager, aerospike.NewNamespaceValidator(clientManager)
}

func initLogger(config *model.Config) *slog.Logger {
	logger := slog.New(log.NewHandler(config.ServiceConfig.GetLoggerOrDefault()))
	slog.SetDefault(logger)
	configStr, _ := decoder.Marshal(dto.NewConfigFromModel(config), decoder.JSON, true)
	slog.Info("Aerospike Backup Service",
		slog.String("version", backup.Version),
		slog.String("commit", backup.CommitHash),
		slog.String("buildTime", backup.BuildTime),
		slog.String("config", string(configStr)))

	return logger
}
