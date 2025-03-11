package service

import (
	"context"
	"fmt"
	"time"
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

	lastFullBackup, err := backend.FindLastFullBackup(ctx, toTime)
	if err != nil {
		return nil, fmt.Errorf("failed retrieve configuration: %w", err)
	}

	return backend.ReadClusterConfiguration(ctx, getConfigurationPath(routine, lastFullBackup))
}
