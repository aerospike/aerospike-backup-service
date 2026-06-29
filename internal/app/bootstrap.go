package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	backup "github.com/aerospike/aerospike-backup-service/v3"
	"github.com/aerospike/aerospike-backup-service/v3/internal/log"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/configuration"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
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

// backupTree holds the backup-mode-specific object graph.
type backupTree struct {
	registry        service.RunningBackupsRegistry
	backupScheduler *service.BackupScheduler
}

// InitComponents builds the full object graph and returns scheduler and HTTP service.
func InitComponents(
	ctx context.Context,
	configFile string,
	remote bool,
) (quartz.Scheduler, *handlers.Service, error) {
	resolver := secrets.NewResolver(ctx)
	s3Accessor := storage.NewS3StorageAccessor(ctx, resolver)
	operations := newStorageOperations(ctx, resolver, s3Accessor)
	clientManager, nsValidator := newAerospikeLayer(resolver)

	config, configurationManager, err := configuration.Load(ctx, configFile, remote, nsValidator, operations)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	appLogger := initLogger(config)

	scheduler, err := service.NewScheduler(ctx, appLogger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	pathService := service.NewPathService(config.ServiceConfig.GetBackupCommonOrDefault().TimestampFormat)
	backendService := service.NewBackupBackendService(pathService, operations)
	var routineStorage u.LockMap

	var tree backupTree
	switch config.ServiceConfig.GetBackupCommonOrDefault().GetBackupModeOrDefault() {
	case model.BackupModeServer:
		tree = newServerBackupTree(scheduler, config, s3Accessor, clientManager, resolver)
	default:
		tree = newScanBackupTree(
			scheduler,
			config,
			pathService,
			backendService,
			clientManager,
			operations,
			&routineStorage,
		)
	}

	configApplier := service.NewDefaultConfigApplier(tree.backupScheduler, tree.registry, config)

	err = configApplier.ApplyNewConfig(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to apply new config: %w", err)
	}

	restoreJobs := service.NewRestoreJobsHolder()
	restoreValidator := service.NewRestoreValidator(tree.registry, config)

	restoreMgr := service.NewRestoreManager(
		restoreexecutor.NewRestore(operations),
		clientManager,
		restoreJobs,
		backendService,
		&routineStorage,
		restoreValidator,
	)

	prometheus.NewMetricsCollector(tree.registry, restoreJobs).Start(ctx, 1*time.Second)

	configRetriever := service.NewConfigRetriever(backendService, pathService, operations)
	httpService := handlers.NewService(
		ctx,
		config,
		configApplier,
		tree.backupScheduler,
		restoreMgr,
		configRetriever,
		backendService,
		tree.registry,
		configurationManager,
		nsValidator,
	)

	return scheduler, httpService, nil
}

func newScanBackupTree(
	scheduler quartz.Scheduler,
	config *model.Config,
	pathService service.PathService,
	backendService service.BackupReaderWriter,
	clientManager aerospike.ClientManager,
	operations *storage.Operations,
	routineStorage *u.LockMap,
) backupTree {
	history := service.NewHistoryManager(backendService)
	registry := service.NewRunningBackupsRegistry(history, config)

	retentionManager := service.NewBackupRetentionManager(backendService, routineStorage)
	clusterConfigWriter := service.NewClusterConfigWriter(clientManager, pathService, operations)
	completionHandler := service.NewBackupCompletionHandler(registry, retentionManager, clusterConfigWriter)

	startController := service.NewStartController(registry, service.NewStartDecider())

	scanBackupExecutor := backupexecutor.NewScanBackupExecutor(clientManager, operations)
	namespaceRunner := service.NewNamespaceBackupRunner(scanBackupExecutor, backendService, pathService)
	namespaceResolver := aerospike.NewNamespaceResolver(clientManager)
	routineRunner := service.NewRoutineBackupRunner(namespaceRunner, namespaceResolver)

	backupOrchestrator := service.NewBackupOrchestrator(
		registry,
		completionHandler,
		service.NewBackupReporter(),
		startController,
		routineRunner,
	)

	return backupTree{
		registry:        registry,
		backupScheduler: service.NewBackupScheduler(scheduler, backupOrchestrator),
	}
}

func newServerBackupTree(
	scheduler quartz.Scheduler,
	config *model.Config,
	s3Accessor *storage.S3StorageAccessor,
	clientManager aerospike.ClientManager,
	resolver secrets.Resolver,
) backupTree {
	history := serverbackup.NewHistoryManager(
		serverbackup.NewS3ListerFactory(serverbackup.NewS3StorageClientProvider(s3Accessor)),
	)
	registry := service.NewRunningBackupsRegistry(history, config)

	completionHandler := service.NewBackupCompletionHandler(
		registry,
		service.NewNopRetentionManager(),
		service.NewNopClusterConfigWriter(),
	)

	startController := serverbackup.NewStartController(serverbackup.NewGate())

	nsRunner := serverbackup.NewNamespaceRunner(clientManager, resolver)
	namespaceResolver := aerospike.NewNamespaceResolver(clientManager)
	routineRunner := serverbackup.NewRoutineRunner(nsRunner, namespaceResolver)

	backupOrchestrator := service.NewBackupOrchestrator(
		registry,
		completionHandler,
		service.NewBackupReporter(),
		startController,
		routineRunner,
	)

	return backupTree{
		registry:        registry,
		backupScheduler: service.NewBackupScheduler(scheduler, backupOrchestrator),
	}
}

func newStorageOperations(
	ctx context.Context,
	resolver secrets.Resolver,
	s3Accessor *storage.S3StorageAccessor,
) *storage.Operations {
	return storage.NewOperations(
		s3Accessor,
		storage.NewGcpStorageAccessor(ctx, resolver),
		storage.NewAzureStorageAccessor(ctx, resolver),
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
	configStr, _ := json.Marshal(dto.NewConfigFromModel(config))
	slog.Info("Aerospike Backup Service",
		slog.String("version", backup.Version),
		slog.String("commit", backup.CommitHash),
		slog.String("buildTime", backup.BuildTime),
		slog.String("config", string(configStr)))

	return logger
}
