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
)

// storageDataWriter is the part of storage.Operations used to write one whole file.
type storageDataWriter interface {
	// WriteDataFile writes a data file to the specified storage.
	WriteDataFile(ctx context.Context, storage model.Storage, fileName string, content []byte) error
}

// ClusterConfigWriter saves the configuration of a routine's source cluster: one file per node,
// at the path [PathService] returns for that routine and timestamp.
type ClusterConfigWriter interface {
	// Write writes the cluster configuration for the given routine and timestamp.
	Write(ctx context.Context, routine *model.BackupRoutine, timestamp time.Time) error
}

type clusterConfigWriter struct {
	pathService   PathService
	storageWriter storageDataWriter
	configSource  aerospike.ClusterConfigSource
}

var _ ClusterConfigWriter = (*clusterConfigWriter)(nil)

// NewClusterConfigWriter returns a ClusterConfigWriter.
func NewClusterConfigWriter(
	pathService PathService,
	storageWriter storageDataWriter,
	configSource aerospike.ClusterConfigSource,
) ClusterConfigWriter {
	return &clusterConfigWriter{
		pathService:   pathService,
		storageWriter: storageWriter,
		configSource:  configSource,
	}
}

func (w *clusterConfigWriter) Write(
	ctx context.Context,
	routine *model.BackupRoutine,
	timestamp time.Time,
) error {
	logger := slog.Default().With(attr.Routine(routine.Name))
	logger.Info("writing cluster config", slog.Time("timestamp", timestamp))

	infos, err := w.configSource.NodeConfigs(ctx, routine.SourceCluster, logger)
	if err != nil {
		return err
	}

	var errs error
	for i, info := range infos {
		confFilePath := w.pathService.GetConfigurationFilePath(routine.Name, timestamp, i)
		err := w.storageWriter.WriteDataFile(ctx, routine.Storage, confFilePath, []byte(info))
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to write cluster configuration backup: %w", err))
			continue
		}

		logger.Debug("Wrote cluster configuration backup", slog.String("path", confFilePath))
	}

	return errs
}
