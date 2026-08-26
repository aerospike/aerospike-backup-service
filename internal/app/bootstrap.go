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
type Components struct {
	Scheduler        quartz.Scheduler
	ServerHTTP       server.HTTP
	ServerHTTPS      server.HTTP
	MetricsCollector *prometheus.MetricsCollector
}

// InitComponents builds the full object graph.
// Components are wired but not started:
// the caller decides when to run them and when to stop them.
//
//nolint:funlen // deliberately keep all initialization in a single function.
func InitComponents(
	ctx context.Context,
	configFile string,
	remote bool,
) (*Components, error) {
	resolver := secrets.NewResolver()
	operations := newStorageOperations(resolver)
	clientManager, nsValidator := newAerospikeLayer(resolver)

	config, configurationManager, err := configuration.Load(ctx, configFile, remote, nsValidator, operations)
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
	httpService := handlers.NewService(
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
	)
	var serverHTTP server.HTTP
	if config.ServiceConfig.ServerHTTP == nil || !config.ServiceConfig.ServerHTTP.GetDisabledOrDefault() {
		serverHTTP = server.NewServerHTTP(ctx, config.ServiceConfig.GetServerHTTPOrDefault(), httpService)
	}
	var serverHTTPS server.HTTP
	if config.ServiceConfig.ServerHTTPS != nil && !config.ServiceConfig.ServerHTTPS.GetDisabledOrDefault() {
		serverHTTPS, err = server.NewServerHTTPS(ctx, config.ServiceConfig.ServerHTTPS, httpService)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTPS server: %w", err)
		}
	}

	return &Components{
		Scheduler:        scheduler,
		ServerHTTP:       serverHTTP,
		ServerHTTPS:      serverHTTPS,
		MetricsCollector: metricsCollector,
	}, nil
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
