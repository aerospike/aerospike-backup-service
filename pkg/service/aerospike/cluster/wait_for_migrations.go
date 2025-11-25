package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/backup-go"
)

const maxDuration = 1 * time.Minute

// WaitForMigrations requests cluster info and waits until all migrations are completed.
func WaitForMigrations(ctx context.Context, ic backup.InfoGetter, namespace string) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, maxDuration)
	defer cancel()

	logger := slog.Default().With("namespace", namespace)
	// Initial immediate check before starting the ticker.
	totalPending, err := ic.GetPendingMigrations(ctxWithTimeout, namespace)
	if err != nil {
		// Log error but continue with periodic checks (cluster might be unstable during rebalance).
		logger.Warn("Failed to fetch initial migration stats, retrying periodically...", slog.Any("error", err))
	} else if totalPending == 0 {
		logger.Debug("Cluster is stable. No migrations in progress.")
		return nil
	} else {
		logger.Debug("Migrations active on initial check", slog.Uint64("pending", totalPending))
	}

	// Wait for migrations to complete with periodic checks.
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	logger.Debug("Waiting for cluster migrations to complete")

	for {
		select {
		case <-ctxWithTimeout.Done():
			return fmt.Errorf("waiting for migrations in namespace %q: %w", namespace, ctxWithTimeout.Err())

		case <-ticker.C:
			totalPending, err := ic.GetPendingMigrations(ctxWithTimeout, namespace)
			if err != nil {
				// Log error but retry (cluster might be unstable during rebalance).
				logger.Warn("Failed to fetch migration stats", slog.Any("error", err))
				continue
			}

			if totalPending == 0 {
				logger.Debug("Cluster is stable. No migrations remaining.")
				return nil
			}

			logger.Debug("Migrations active", slog.Uint64("pending", totalPending))
		}
	}
}
