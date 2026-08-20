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

type storageDataWriter interface {
	// WriteDataFile writes a data file to the specified storage.
	WriteDataFile(ctx context.Context, storage model.Storage, fileName string, content []byte) error
}

// ClusterConfigWriter writes cluster configuration backups to storage.
type ClusterConfigWriter interface {
	// Write writes the cluster configuration for the given routine and timestamp.
	Write(ctx context.Context, routine *model.BackupRoutine, timestamp time.Time) error
}

// DefaultClusterConfigWriter is the default implementation of ClusterConfigWriter.
type DefaultClusterConfigWriter struct {
	pathService  PathService
	operations   storageDataWriter
	configSource aerospike.ClusterConfigSource
}

// NewClusterConfigWriter returns a new DefaultClusterConfigWriter instance.
func NewClusterConfigWriter(
	pathService PathService,
	operations storageDataWriter,
	configSource aerospike.ClusterConfigSource,
) *DefaultClusterConfigWriter {
	return &DefaultClusterConfigWriter{
		pathService:  pathService,
		operations:   operations,
		configSource: configSource,
	}
}

func (w *DefaultClusterConfigWriter) Write(
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
		err := w.operations.WriteDataFile(ctx, routine.Storage, confFilePath, []byte(info))
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to write cluster configuration backup: %w", err))
			continue
		}

		logger.Debug("Wrote cluster configuration backup", slog.String("path", confFilePath))
	}

	return errs
}
