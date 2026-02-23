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
)

type storageDataWriter interface {
	// WriteDataFile writes a data file to the specified storage.
	WriteDataFile(ctx context.Context, storage model.Storage, fileName string, content []byte) error
}

// ClusterConfigWriter handles writing cluster configuration to storage.
type ClusterConfigWriter interface {
	// Write writes the cluster configuration for the given routine and timestamp.
	Write(ctx context.Context, routine *model.BackupRoutine, timestamp time.Time) error
}

// DefaultClusterConfigWriter is the default implementation of ClusterConfigWriter.
type DefaultClusterConfigWriter struct {
	clientManager aerospike.ClientManager
	pathService   PathService
	operations    storageDataWriter
}

// NewClusterConfigWriter returns a new DefaultClusterConfigWriter instance.
func NewClusterConfigWriter(
	clientManager aerospike.ClientManager,
	pathService PathService,
	operations storageDataWriter,
) *DefaultClusterConfigWriter {
	return &DefaultClusterConfigWriter{
		clientManager: clientManager,
		pathService:   pathService,
		operations:    operations,
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
		return errors.New("failed to read Aerospike configuration")
	}

	var errs error
	for i, info := range infos {
		confFilePath := w.pathService.GetConfigurationFilePath(routine.Name, timestamp, i)
		err := w.operations.WriteDataFile(ctx, routine.Storage, confFilePath, []byte(info))
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to write cluster configuration backup: %w", err))
			continue
		}

		logger.Debug("Wrote cluster configuration backup", slog.String("path", confFilePath))
	}

	return errs
}
