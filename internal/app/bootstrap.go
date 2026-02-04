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
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	u "github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/reugn/go-quartz/quartz"
)

// InitComponents builds the full object graph and returns config, scheduler, HTTP service, and logger.
func InitComponents(
	ctx context.Context,
	configFile string,
	remote bool,
) (quartz.Scheduler, *handlers.Service, error) {
	resolver := secrets.NewResolver(ctx)
	operations := newStorageOperations(ctx, resolver)
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
	backendService := service.NewBackupBackendService(config, pathService, operations)
	history := service.NewHistoryManager(backendService)
	registry := service.NewRunningBackupsRegistry(history, config)
	var routineStorage u.LockMap
	retentionManager := service.NewBackupRetentionManager(backendService, &routineStorage)
	clusterConfigWriter := service.NewClusterConfigWriter(clientManager, pathService, operations)
	completionHandler := service.NewBackupCompletionHandler(registry, retentionManager, clusterConfigWriter)
	backupExecutor := backupexecutor.NewDefaultBackupExecutor(operations)
	backupComponents := service.NewBackupComponents(
		clientManager, backupExecutor, registry, completionHandler, backendService,
	)
	configApplier := service.NewDefaultConfigApplier(scheduler, registry, backupComponents, config, pathService)

	err = configApplier.ApplyNewConfig(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to apply new config: %w", err)
	}

	restoreMgr, restoreJobs := newRestoreManager(operations, clientManager, backendService, &routineStorage)
	service.NewMetricsCollector(registry, restoreJobs).Start(ctx, 1*time.Second)

	configRetriever := service.NewConfigRetriever(backendService, pathService, operations)
	httpService := handlers.NewService(
		ctx,
		config,
		configApplier,
		scheduler,
		restoreMgr,
		configRetriever,
		backendService,
		registry,
		configurationManager,
		nsValidator,
	)

	return scheduler, httpService, nil
}

func newStorageOperations(ctx context.Context, resolver secrets.Resolver) *storage.Operations {
	return storage.NewOperations(
		storage.NewS3StorageAccessor(ctx, resolver),
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

func newRestoreManager(
	operations *storage.Operations,
	clientManager aerospike.ClientManager,
	backendService service.BackupReader,
	routineStorage *u.LockMap,
) (service.RestoreManager, *service.RestoreJobsHolder) {
	restoreJobs := service.NewRestoreJobsHolder()
	restoreMgr := service.NewRestoreManager(
		restoreexecutor.NewRestore(operations),
		clientManager,
		restoreJobs,
		backendService,
		routineStorage,
	)

	return restoreMgr, restoreJobs
}
