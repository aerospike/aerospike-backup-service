package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike/cluster"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// ClusterConfigWriter handles writing cluster configuration to storage.
type ClusterConfigWriter interface {
	// Write writes the cluster configuration for the given routine and timestamp.
	Write(ctx context.Context, routine *model.BackupRoutine, timestamp time.Time) error
}

// DefaultClusterConfigWriter is the default implementation of ClusterConfigWriter.
type DefaultClusterConfigWriter struct {
	clientManager  aerospike.ClientManager
	pathService    PathService
	storageService storage.Service
}

// NewClusterConfigWriter returns a new DefaultClusterConfigWriter instance.
func NewClusterConfigWriter(
	clientManager aerospike.ClientManager,
	pathService PathService,
	storageService storage.Service,
) *DefaultClusterConfigWriter {
	return &DefaultClusterConfigWriter{
		clientManager:  clientManager,
		pathService:    pathService,
		storageService: storageService,
	}
}

func (w *DefaultClusterConfigWriter) Write(
	ctx context.Context,
	routine *model.BackupRoutine,
	timestamp time.Time,
) error {
	logger := slog.Default().With(attr.Routine(routine.Name))

	client, err := w.clientManager.GetClient(ctx, routine.SourceCluster, logger)
	if err != nil {
		return fmt.Errorf("cannot get backup client: %w", err)
	}

	defer w.clientManager.Close(client)

	infos := cluster.ReadConfiguration(client.AerospikeClient(), logger)
	if len(infos) == 0 {
		return errors.New("could not read aerospike configuration")
	}

	for i, info := range infos {
		confFilePath := w.pathService.GetConfigurationFilePath(routine.Name, timestamp, i)
		err := w.storageService.WriteDataFile(ctx, routine.Storage, confFilePath, []byte(info))
		if err != nil {
			logger.Error("Failed to write cluster configuration backup",
				attr.Error(err))
		}
		logger.Debug("Wrote cluster configuration backup", slog.String("path", confFilePath))
	}

	return nil
}
