package app

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
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
	completionHandler := service.NewBackupCompletionHandler(registry, retentionManager, clusterConfigWriter)
	backupExecutor := backupexecutor.NewDefaultBackupExecutor(operations)
	backupComponents := service.NewBackupComponents(
		clientManager, backupExecutor, registry, completionHandler, backendService,
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
