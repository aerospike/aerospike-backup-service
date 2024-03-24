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

	"github.com/aerospike/backup/internal/server"
	"github.com/aerospike/backup/internal/util"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/aerospike/backup/pkg/stdio"
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
		Version: util.Version,
		PreRunE: validateFlags,
	}

	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "configuration file path/URL")
	rootCmd.Flags().BoolVarP(&remote, "remote", "r", false, "use remote config file")

	rootCmd.RunE = func(_ *cobra.Command, _ []string) error {
		manager, err := service.NewConfigurationManager(configFile, remote)
		if err != nil {
			return err
		}
		server.ConfigurationManager = manager
		setConfigurationManager(configFile)
		// read configuration file
		config, err := readConfiguration()
		if err != nil {
			return err
		}
		// get system ctx
		ctx := systemCtx()
		// set default loggers
		loggerConfig := config.ServiceConfig.Logger
		slog.SetDefault(slog.New(util.LogHandler(loggerConfig)))
		logger.SetDefault(util.NewQuartzLogger(ctx))
		slog.Info("Aerospike Backup Service", "commit", commit, "buildTime", buildTime)
		// init stderr log capturer
		stdio.Stderr = stdio.NewCgoStdio(config.ServiceConfig.Logger.CaptureShared)
		// schedule all configured backups
		backends := service.NewBackupBackends(config)
		scheduler, err := service.ScheduleBackup(ctx, config, backends)
		if err != nil {
			return err
		}
		// run HTTP server
		err = runHTTPServer(ctx, backends, config, scheduler)
		// shutdown shared resources
		shared.Shutdown()
		// stop the scheduler
		scheduler.Stop()
		return err
	}

	err := rootCmd.Execute()
	if err != nil {
		slog.Error("Error in rootCmd.Execute", "err", err)
	}

	return util.ToExitVal(err)
}

func setConfigurationManager(configFile string) {
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

func readConfiguration() (*model.Config, error) {
	config, err := server.ConfigurationManager.ReadConfiguration()
	if err != nil {
		slog.Error("failed to read configuration file", "error", err)
		return nil, err
	}
	if err = config.Validate(); err != nil {
		return nil, err
	}
	slog.Info(fmt.Sprintf("Configuration: %v", *config))
	return config, nil
}

func runHTTPServer(ctx context.Context, backends service.BackendsHolder,
	config *model.Config, scheduler quartz.Scheduler) error {
	server := server.NewHTTPServer(backends, config, scheduler)
	go func() {
		server.Start()
	}()

	<-ctx.Done()
	time.Sleep(time.Millisecond * 100) // wait for other goroutines to exit
	// shutdown the HTTP server gracefully
	if err := server.Shutdown(); err != nil {
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
