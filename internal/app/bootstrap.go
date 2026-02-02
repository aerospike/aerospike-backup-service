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
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/reugn/go-quartz/quartz"
)

// InitComponents builds the full object graph and returns config, scheduler, HTTP service, and logger.
func InitComponents(
	ctx context.Context,
	configFile string,
	remote bool,
) (*model.Config, quartz.Scheduler, *handlers.Service, *slog.Logger, error) {
	resolver := secrets.NewResolver(ctx)
	operations := newStorageOperations(ctx, resolver)
	clientManager, nsValidator := newAerospikeLayer(resolver)

	config, configurationManager, err := configuration.Load(ctx, configFile, remote, nsValidator, operations)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	appLogger := setDefaultLogger(config.ServiceConfig.GetLoggerOrDefault())
	logConfigOnce(config)

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

func logConfigOnce(config *model.Config) {
	configStr, _ := json.Marshal(dto.NewConfigFromModel(config))
	slog.Info("Aerospike Backup Service",
		slog.String("version", backup.Version),
		slog.String("commit", backup.CommitHash),
		slog.String("buildTime", backup.BuildTime),
		slog.String("config", string(configStr)))
}

func setDefaultLogger(loggerConfig *model.LoggerConfig) *slog.Logger {
	appLogger := slog.New(
		log.NewHandler(loggerConfig),
	)
	slog.SetDefault(appLogger)
	return appLogger
}
