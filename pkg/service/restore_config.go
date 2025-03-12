package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// configRetriever is used to read Aerospike configuration from backup.
type configRetriever struct {
	backends BackendsHolder
}

// RetrieveConfiguration return backed up Aerospike configuration.
func (cr *configRetriever) RetrieveConfiguration(ctx context.Context, routine string, toTime *time.Time,
) ([]byte, error) {
	backend, found := cr.backends.GetReader(routine)
	if !found {
		return nil, fmt.Errorf("%w: routine %s", errBackendNotFound, routine)
	}

	lastFullBackup, err := backend.LastBackupTime(ctx, model.TimeBounds{ToTime: toTime}, jobTypeFull)
	if err != nil {
		return nil, fmt.Errorf("failed find last backup: %w", err)
	}

	path := getConfigurationPath(routine, lastFullBackup)
	slog.Info("getConfiguration", slog.String("routine", routine), slog.String("path", path))
	return backend.ReadClusterConfiguration(ctx, path)
}
