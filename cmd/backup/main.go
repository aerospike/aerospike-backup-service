package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	backup "github.com/aerospike/aerospike-backup-service/v3"
	"github.com/aerospike/aerospike-backup-service/v3/internal/app"
	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/internal/log"
	"github.com/aerospike/aerospike-backup-service/v3/internal/server"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/prometheus"
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

	components, err := app.InitComponents(ctx, configFile, remote)
	if err != nil {
		return err
	}

	components.Scheduler.Start(ctx)
	components.MetricsCollector.Start(ctx, prometheus.CollectInterval)
	components.CertReloader.Start(ctx) // watchers; initial Load already happened in Init

	err = server.Run(ctx, components.Servers)

	// stop the scheduler
	components.Scheduler.Stop()

	return err
}

func systemCtx() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
}

func main() {
	// start the application
	os.Exit(run())
}
