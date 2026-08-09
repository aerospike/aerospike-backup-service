package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	backup "github.com/aerospike/aerospike-backup-service/v3"
	"github.com/aerospike/aerospike-backup-service/v3/internal/app"
	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/internal/log"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server/handlers"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/audit"
	"github.com/spf13/cobra"
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
		slog.String("commit", backup.CommitHash),
		slog.String("buildTime", backup.BuildTime))

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
		slog.Error("Command execution failed", attr.Error(err))
	}

	return log.ToExitVal(err)
}

func startService(configFile string, remote bool) error {
	ctx, stop := systemCtx()
	defer stop()

	scheduler, httpService, auditor, err := app.InitComponents(ctx, configFile, remote)
	if err != nil {
		return err
	}

	// start the scheduler only after all the initialization is done
	scheduler.Start(ctx)

	err = runHTTPServer(ctx, httpService, auditor)

	// stop the scheduler
	scheduler.Stop()

	return err
}

func systemCtx() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
}

func runHTTPServer(ctx context.Context, service *handlers.Service, auditor *audit.SimpleAuditor) error {
	httpServer := server.NewHTTPServer(service)

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

	if err := httpServer.Shutdown(); err != nil {
		slog.Error("HTTP server shutdown failed", attr.Error(err))
		return err
	}

	auditor.WriteEvent(context.Background(), "ServerShutdown", audit.StatusSuccess)
	slog.Info("HTTP server shut down gracefully")

	return nil
}

func main() {
	// start the application
	os.Exit(run())
}
