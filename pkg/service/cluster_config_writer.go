package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// ClusterConfigWriter handles writing cluster configuration to storage.
type ClusterConfigWriter interface {
	Write(ctx context.Context, routineName string, timestamp time.Time)
}

// DefaultClusterConfigWriter is the default implementation of ClusterConfigWriter.
type DefaultClusterConfigWriter struct {
	config        *model.Config
	clientManager aerospike.ClientManager
}

// NewClusterConfigWriter returns a new DefaultClusterConfigWriter instance.
func NewClusterConfigWriter(clientManager aerospike.ClientManager, config *model.Config) *DefaultClusterConfigWriter {
	return &DefaultClusterConfigWriter{
		config:        config,
		clientManager: clientManager,
	}
}

func (w *DefaultClusterConfigWriter) Write(
	ctx context.Context,
	routineName string,
	timestamp time.Time,
) {
	logger := slog.Default().With(slog.String("routine", routineName))
	routine, _ := w.config.Routine(routineName)

	client, _ := w.clientManager.GetClient(routine.SourceCluster)
	defer w.clientManager.Close(client)

	infos := aerospike.ScanClusterConfiguration(client.AerospikeClient(), logger)
	if len(infos) == 0 {
		logger.Warn("Could not read aerospike configuration")
		return
	}

	for i, info := range infos {
		confFilePath := getConfigurationFilePath(routineName, timestamp, i)
		err := storage.WriteDataFile(ctx, routine.Storage, confFilePath, []byte(info))
		if err != nil {
			logger.Error("Failed to write cluster configuration backup",
				slog.Any("err", err))
		}
		logger.Debug("Wrote cluster configuration backup", slog.String("path", confFilePath))
	}
}
