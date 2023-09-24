package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"github.com/aerospike/backup/internal/server"
	"github.com/aerospike/backup/internal/util"
	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/service"
	"github.com/spf13/cobra"
)

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
			slog.Error("failed to read configuration file", "error", err)
			exitVal = 1
			return
		}
		slog.Info(fmt.Sprintf("Configuration: %v", *config))
		// get system ctx
		ctx := systemCtx()
		// schedule all configured backups
		handlers := service.BuildBackupHandlers(config)
		service.ScheduleHandlers(ctx, handlers)
		// run HTTP server
		exitVal = runHTTPServer(ctx, host, port, config)
	}

	rootCmd.Flags().StringVar(&host, "host", "0.0.0.0", "service host")
	rootCmd.Flags().IntVar(&port, "port", 8080, "service port")
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "configuration file path")

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Error in rootCmd.Execute", "err", err)
		exitVal = 1
	}

	return exitVal
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

// run HTTP server
func runHTTPServer(ctx context.Context, host string, port int, config *model.Config) int {
	server := server.NewHTTPServer(host, port, config)
	go func() {
		server.Start()
	}()

	<-ctx.Done()
	time.Sleep(time.Millisecond * 100) // wait for other goroutines to exit
	// shutdown the HTTP server gracefully
	if err := server.Shutdown(); err != nil {
		slog.Error("HTTP server shutdown failed", "error", err)
		return 1
	}

	slog.Info("HTTP server shutdown gracefully")
	return 0
}

func main() {
	// set default logger
	slog.SetDefault(slog.New(util.LogHandler))

	// start the application
	os.Exit(run())
}
