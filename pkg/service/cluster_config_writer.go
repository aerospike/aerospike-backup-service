package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
)

// DefaultClusterConfigWriter is the default implementation of ClusterConfigWriter.
type DefaultClusterConfigWriter struct {
	storage     model.Storage
	routineName string
	policy      *model.BackupPolicy
	logger      *slog.Logger
}

// NewClusterConfigWriter returns a new DefaultClusterConfigWriter instance.
func NewClusterConfigWriter(
	storage model.Storage,
	routineName string,
	policy *model.BackupPolicy,
	logger *slog.Logger,
) *DefaultClusterConfigWriter {
	return &DefaultClusterConfigWriter{
		storage:     storage,
		routineName: routineName,
		policy:      policy,
		logger:      logger,
	}
}

func (w *DefaultClusterConfigWriter) Write(
	ctx context.Context,
	client aerospike.Cluster,
	timestamp time.Time,
) {
	infos := aerospike.ScanClusterConfiguration(client, w.logger)
	if len(infos) == 0 {
		w.logger.Warn("Could not read aerospike configuration")
		return
	}

	for i, info := range infos {
		confFilePath := getConfigurationPath(w.routineName, timestamp, i)
		err := storage.WriteFile(ctx, w.storage, confFilePath, []byte(info))
		if err != nil {
			w.logger.Error("Failed to write cluster configuration backup",
				slog.Any("err", err))
		}
		w.logger.Debug("Wrote cluster configuration backup", slog.String("path", confFilePath))
	}
}
