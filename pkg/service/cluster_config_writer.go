package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike/cluster"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// ClusterConfigWriter handles writing cluster configuration to storage.
type ClusterConfigWriter interface {
	Write(ctx context.Context, routineName string, timestamp time.Time) error
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
) error {
	logger := slog.Default().With(slog.String("routine", routineName))
	routine, found := w.config.Routine(routineName)
	if !found {
		return fmt.Errorf("routine not found: %q", routineName)
	}

	client, err := w.clientManager.GetClient(routine.SourceCluster)
	if err != nil {
		return fmt.Errorf("cannot get backup client: %w", err)
	}

	defer w.clientManager.Close(client)

	infos := cluster.ReadConfiguration(client.AerospikeClient(), logger)
	if len(infos) == 0 {
		return errors.New("could not read aerospike configuration")
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

	return nil
}
