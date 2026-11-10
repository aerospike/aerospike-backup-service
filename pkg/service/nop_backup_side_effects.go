package service

import (
	"context"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

type nopRetentionManager struct{}

var _ BackupRetentionManager = nopRetentionManager{}

// NopRetentionManager returns a BackupRetentionManager that deletes nothing.
func NopRetentionManager() BackupRetentionManager {
	return nopRetentionManager{}
}

func (nopRetentionManager) ApplyRetention(context.Context, *model.BackupRoutine) error {
	return nil
}

type nopClusterConfigWriter struct{}

var _ ClusterConfigWriter = nopClusterConfigWriter{}

// NopClusterConfigWriter returns a ClusterConfigWriter that writes nothing.
func NopClusterConfigWriter() ClusterConfigWriter {
	return nopClusterConfigWriter{}
}

func (nopClusterConfigWriter) Write(context.Context, *model.BackupRoutine, time.Time) error {
	return nil
}

type noConfigRetriever struct{}

var _ ConfigRetriever = noConfigRetriever{}

// NoConfigRetriever returns a ConfigRetriever for a backup mode that does not save the cluster
// configuration. Every retrieval fails with [model.ErrNotFound].
func NoConfigRetriever() ConfigRetriever {
	return noConfigRetriever{}
}

func (noConfigRetriever) RetrieveConfiguration(context.Context, *model.BackupRoutine, time.Time) ([]byte, error) {
	return nil, fmt.Errorf("this backup mode does not save the cluster configuration: %w", model.ErrNotFound)
}
