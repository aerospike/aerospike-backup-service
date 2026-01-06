package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	backup "github.com/aerospike/aerospike-backup-service/v3"
	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/internal/log"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/configuration"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	u "github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/reugn/go-quartz/quartz"
	"github.com/spf13/cobra"
)

var (
	commitHash string
	buildTime  string
)

// run parses the CLI parameters and executes backup.
func run() int {
	var (
		configFile string
		remote     bool
	)

	// Log commit information as the first log entry, ensuring it appears at the top
	// regardless of subsequent errors or execution flow.
	slog.Info("Aerospike Backup Service",
		slog.String("version", backup.Version),
		slog.String("commit", commitHash),
		slog.String("buildTime", buildTime))

	validateFlags := func(_ *cobra.Command, _ []string) error {
		if len(configFile) == 0 {
			return errors.New("--config is required")
		}
		return nil
	}

	rootCmd := &cobra.Command{
		Use:     "aerospike-backup-service",
		Short:   "Aerospike Backup Service",
		Version: backup.Version,
		PreRunE: validateFlags,
	}

	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "configuration file path/URL")
	rootCmd.Flags().BoolVarP(&remote, "remote", "r", false, "use remote config file")

	rootCmd.RunE = func(_ *cobra.Command, _ []string) error {
		return startService(configFile, remote)
	}

	err := rootCmd.Execute()
	if err != nil {
		slog.Error("Error in rootCmd.Execute", attr.Error(err))
	}

	return log.ToExitVal(err)
}

func startService(configFile string, remote bool) error {
	ctx := systemCtx()
	config, scheduler, httpService, appLogger, err := initComponents(ctx, configFile, remote)
	if err != nil {
		return err
	}

	// start the scheduler only after all the initialization is done
	scheduler.Start(ctx)

	// run HTTP server
	err = runHTTPServer(ctx, config.ServiceConfig.GetHTTPServerOrDefault(), httpService, appLogger)

	// stop the scheduler
	scheduler.Stop()

	return err
}

//nolint:funlen
func initComponents(ctx context.Context, configFile string, remote bool) (
	*model.Config, quartz.Scheduler, *handlers.Service, *slog.Logger, error,
) {
	clientManager := aerospike.NewClientManager(aerospike.NewClientFactory(), 10*time.Second)
	nsValidator := aerospike.NewNamespaceValidator(clientManager)

	config, configurationManager, err := configuration.Load(ctx, configFile, remote, nsValidator)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	appLogger := setDefaultLogger(config.ServiceConfig.GetLoggerOrDefault())

	// Re-log build metadata now that the app-logger is active.
	// Duplication with the bootstrap log is intentional.
	configStr, _ := json.Marshal(dto.NewConfigFromModel(config))
	slog.Info("Aerospike Backup Service",
		slog.String("version", backup.Version),
		slog.String("commit", commitHash),
		slog.String("buildTime", buildTime),
		slog.String("config", string(configStr)))

	// schedule all configured backup routines
	scheduler, err := service.NewScheduler(ctx, appLogger)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	pathService := service.NewPathService(config.ServiceConfig.GetBackupCommonOrDefault().TimestampFormat)
	backendService := service.NewBackupBackendService(config, pathService)
	history := service.NewHistoryManager(backendService)
	registry := service.NewRunningBackupsRegistry(history, config)

	var routineStorage u.LockMap
	retentionManager := service.NewBackupRetentionManager(backendService, &routineStorage)
	clusterConfigWriter := service.NewClusterConfigWriter(clientManager, pathService)
	backupExecutor := backupexecutor.NewDefaultBackupExecutor()
	backupComponents := service.NewBackupComponents(
		clientManager, backupExecutor, registry, retentionManager,
		backendService, clusterConfigWriter)
	configApplier := service.NewDefaultConfigApplier(scheduler, registry, backupComponents, config, pathService)

	err = configApplier.ApplyNewConfig(ctx)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to apply new config: %w", err)
	}

	var restoreJobs = service.NewRestoreJobsHolder()
	service.NewMetricsCollector(registry, restoreJobs).Start(ctx, 1*time.Second)

	restoreMgr := service.NewRestoreManager(
		restoreexecutor.NewRestore(), clientManager, restoreJobs, backendService, &routineStorage)

	configRetriever := service.NewConfigRetriever(backendService, pathService)
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

	return config, scheduler, httpService, appLogger, nil
}

func setDefaultLogger(loggerConfig *model.LoggerConfig) *slog.Logger {
	appLogger := slog.New(
		log.NewHandler(loggerConfig),
	)
	slog.SetDefault(appLogger)
	return appLogger
}

func systemCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
		<-sigch
		slog.Debug("Got system signal")
		cancel()
	}()

	return ctx
}

func runHTTPServer(
	ctx context.Context, serverConfig *model.HTTPServerConfig, service *handlers.Service, logger *slog.Logger,
) error {
	httpServer := server.NewHTTPServer(serverConfig, service, logger)

	// Channel to capture server startup errors
	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.Start(); err != nil {
			errCh <- err
		}
	}()

	// Wait for either context cancellation or server error
	select {
	case err := <-errCh:
		return fmt.Errorf("HTTP server failed: %w", err)
	case <-ctx.Done():
	}

	time.Sleep(time.Millisecond * 100) // wait for other goroutines to exit
	// shutdown the HTTP server gracefully
	if err := httpServer.Shutdown(); err != nil {
		slog.Error("HTTP server shutdown failed", attr.Error(err))
		return err
	}

	slog.Info("HTTP server shutdown gracefully")

	return nil
}

func main() {
	// start the application
	os.Exit(run())
}
