package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"log/slog"

	"github.com/aerospike/backup/internal/server"
	"github.com/aerospike/backup/internal/util"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"github.com/spf13/cobra"
)

// main logger
var logger *slog.Logger

// run parses the CLI parameters and executes backup.
func run() int {
	var (
		host, configFile string
		port, exitVal    int
	)

	rootCmd := &cobra.Command{
		Use:     "Use the following properties for service configuration",
		Short:   "Aerospike Backup Service",
		Version: util.Version,
	}

	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		config, err := model.ReadConfiguration(configFile)
		if err != nil {
			logger.Error("failed to read configuration file", "error", err)
			exitVal = 1
			return
		}
		logger.Info(fmt.Sprintf("Configuration: %v", *config))
		// schedule all configured backups
		go service.ScheduleBackupJobs(context.TODO(), config)
		exitVal = runHTTPServer(host, port, config)
	}

	rootCmd.Flags().StringVar(&host, "host", "0.0.0.0", "service host")
	rootCmd.Flags().IntVar(&port, "port", 8080, "service port")
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "configuration file path")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		exitVal = 1
	}

	return exitVal
}

// run HTTP server
func runHTTPServer(host string, port int, config *model.Config) int {
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	server := server.NewHTTPServer(host, port, config)
	go func() {
		server.Start()
	}()

	<-sigch
	// shutdown the HTTP server gracefully
	if err := server.Shutdown(); err != nil {
		logger.Error("HTTP server shutdown failed", "error", err)
		return 1
	}

	logger.Info("HTTP server shutdown gracefully")
	return 0
}

func main() {
	// init logger
	logger = slog.New(util.LogHandler)

	// start the application
	os.Exit(run())
}
