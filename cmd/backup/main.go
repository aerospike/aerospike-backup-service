package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	backup "github.com/aerospike/aerospike-backup-service/v2"
	"github.com/aerospike/aerospike-backup-service/v2/internal/server"
	"github.com/aerospike/aerospike-backup-service/v2/internal/server/configuration"
	"github.com/aerospike/aerospike-backup-service/v2/internal/util"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service"
	"github.com/reugn/go-quartz/logger"
	"github.com/reugn/go-quartz/quartz"
	"github.com/spf13/cobra"
)

var (
	commit    string
	buildTime string
)

// run parses the CLI parameters and executes backup.
func run() int {
	var (
		configFile string
		remote     bool
	)

	validateFlags := func(_ *cobra.Command, _ []string) error {
		if len(configFile) == 0 {
			return errors.New("--config is required")
		}
		return nil
	}

	rootCmd := &cobra.Command{
		Use:     "Use the following properties for service configuration",
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
		slog.Error("Error in rootCmd.Execute", "err", err)
	}

	return util.ToExitVal(err)
}

func startService(configFile string, remote bool) error {
	ctx := systemCtx()

	config, manager, err := configuration.Load(ctx, configFile, remote)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// set default loggers
	loggerConfig := config.ServiceConfig.Logger
	appLogger := slog.New(
		util.LogHandler(loggerConfig),
	)
	slog.SetDefault(appLogger)
	logger.SetDefault(util.NewQuartzLogger(ctx))
	slog.Info("Aerospike Backup Service", "commit", commit, "buildTime", buildTime)

	// schedule all configured backups
	backends := service.NewBackupBackends(config)
	clientManager := service.NewClientManager(&service.DefaultClientFactory{})
	handlers := service.MakeHandlers(clientManager, config, backends)
	scheduler, err := service.ScheduleBackup(ctx, config, handlers)
	if err != nil {
		return err
	}

	var restoreJobs = service.NewRestoreJobsHolder()
	service.NewMetricsCollector(handlers, restoreJobs).Start(ctx, 1*time.Second)

	// run HTTP server
	err = runHTTPServer(ctx, config, scheduler, backends, handlers, manager, clientManager, appLogger, restoreJobs)

	// stop the scheduler
	scheduler.Stop()

	return err
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

func runHTTPServer(ctx context.Context,
	config *model.Config,
	scheduler quartz.Scheduler,
	backends service.BackendsHolder,
	handlerHolder service.BackupHandlerHolder,
	configurationManager configuration.Manager,
	clientManger service.ClientManager,
	logger *slog.Logger,
	restoreJobs *service.RestoreJobsHolder,
) error {
	httpServer := server.NewHTTPServer(
		config,
		scheduler,
		backends,
		handlerHolder,
		configurationManager,
		clientManger,
		logger,
		restoreJobs,
	)
	go func() {
		httpServer.Start()
	}()

	<-ctx.Done()
	time.Sleep(time.Millisecond * 100) // wait for other goroutines to exit
	// shutdown the HTTP server gracefully
	if err := httpServer.Shutdown(); err != nil {
		slog.Error("HTTP server shutdown failed", "error", err)
		return err
	}

	slog.Info("HTTP server shutdown gracefully")
	return nil
}

func main() {
	// start the application
	os.Exit(run())
}
