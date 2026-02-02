package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	backup "github.com/aerospike/aerospike-backup-service/v3"
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

// backupStack holds components built for the backup domain.
type backupStack struct {
	PathService    service.PathService
	BackendService *service.BackupBackendServiceImpl
	Registry       service.RunningBackupsRegistry
	ConfigApplier  service.ConfigApplier
	RoutineStorage *u.LockMap
}

// restoreStack holds components built for the restore domain.
type restoreStack struct {
	RestoreManager    service.RestoreManager
	RestoreJobsHolder *service.RestoreJobsHolder
}

// initComponents builds the full object graph and returns config, scheduler, HTTP service, and logger.
func initComponents(ctx context.Context, configFile string, remote bool) (
	*model.Config, quartz.Scheduler, *handlers.Service, *slog.Logger, error,
) {
	resolver := secrets.NewResolver(ctx)
	operations := newStorageOperations(ctx, resolver)
	clientManager, nsValidator := newAerospikeLayer(resolver)

	config, configurationManager, err := configuration.Load(ctx, configFile, remote, nsValidator, operations)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	appLogger := setDefaultLogger(config.ServiceConfig.GetLoggerOrDefault())
	logConfigOnce(config, appLogger)

	scheduler, err := service.NewScheduler(ctx, appLogger)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	backupStack := newBackupStack(config, clientManager, operations, scheduler)
	err = backupStack.ConfigApplier.ApplyNewConfig(ctx)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to apply new config: %w", err)
	}

	restoreStack := newRestoreStack(operations, clientManager, backupStack.BackendService, backupStack.RoutineStorage)
	service.NewMetricsCollector(backupStack.Registry, restoreStack.RestoreJobsHolder).Start(ctx, 1*time.Second)

	configRetriever := service.NewConfigRetriever(backupStack.BackendService, backupStack.PathService, operations)
	httpService := handlers.NewService(
		ctx,
		config,
		backupStack.ConfigApplier,
		scheduler,
		restoreStack.RestoreManager,
		configRetriever,
		backupStack.BackendService,
		backupStack.Registry,
		configurationManager,
		nsValidator,
	)

	return config, scheduler, httpService, appLogger, nil
}

// newStorageOperations creates storage operations for all backends (S3, GCP, Azure, local).
func newStorageOperations(ctx context.Context, resolver secrets.Resolver) *storage.Operations {
	return storage.NewOperations(
		storage.NewS3StorageAccessor(ctx, resolver),
		storage.NewGcpStorageAccessor(ctx, resolver),
		storage.NewAzureStorageAccessor(ctx, resolver),
		storage.NewLocalStorageAccessor(),
	)
}

// newAerospikeLayer creates the Aerospike client factory, manager, and namespace validator.
func newAerospikeLayer(resolver secrets.Resolver) (aerospike.ClientManager, aerospike.NamespaceValidator) {
	passwordResolver := secrets.NewPasswordResolver(resolver)
	clientFactory := aerospike.NewClientFactory(passwordResolver)
	clientManager := aerospike.NewClientManager(clientFactory, aerospike.DefaultCloseDelay)
	nsValidator := aerospike.NewNamespaceValidator(clientManager)
	return clientManager, nsValidator
}

// newBackupStack builds PathService, BackendService, Registry, ConfigApplier, and shared routine storage.
func newBackupStack(
	config *model.Config,
	clientManager aerospike.ClientManager,
	operations *storage.Operations,
	scheduler quartz.Scheduler,
) backupStack {
	pathService := service.NewPathService(config.ServiceConfig.GetBackupCommonOrDefault().TimestampFormat)
	backendService := service.NewBackupBackendService(config, pathService, operations)
	history := service.NewHistoryManager(backendService)
	registry := service.NewRunningBackupsRegistry(history, config)

	var routineStorage u.LockMap
	retentionManager := service.NewBackupRetentionManager(backendService, &routineStorage)
	clusterConfigWriter := service.NewClusterConfigWriter(clientManager, pathService, operations)
	backupExecutor := backupexecutor.NewDefaultBackupExecutor(operations)
	backupComponents := service.NewBackupComponents(
		clientManager, backupExecutor, registry, retentionManager,
		backendService, clusterConfigWriter,
	)
	configApplier := service.NewDefaultConfigApplier(scheduler, registry, backupComponents, config, pathService)

	return backupStack{
		PathService:    pathService,
		BackendService: backendService,
		Registry:       registry,
		ConfigApplier:  configApplier,
		RoutineStorage: &routineStorage,
	}
}

// newRestoreStack builds RestoreJobsHolder and RestoreManager.
func newRestoreStack(
	operations *storage.Operations,
	clientManager aerospike.ClientManager,
	backendService service.BackupReader,
	routineStorage *u.LockMap,
) restoreStack {
	restoreJobs := service.NewRestoreJobsHolder()
	restoreMgr := service.NewRestoreManager(
		restoreexecutor.NewRestore(operations),
		clientManager,
		restoreJobs,
		backendService,
		routineStorage,
	)
	return restoreStack{
		RestoreManager:    restoreMgr,
		RestoreJobsHolder: restoreJobs,
	}
}

func logConfigOnce(config *model.Config, _ *slog.Logger) {
	configStr, _ := json.Marshal(dto.NewConfigFromModel(config))
	slog.Info("Aerospike Backup Service",
		slog.String("version", backup.Version),
		slog.String("commit", backup.CommitHash),
		slog.String("buildTime", backup.BuildTime),
		slog.String("config", string(configStr)))
}
