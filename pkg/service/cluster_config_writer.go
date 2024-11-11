package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/storage"
	"github.com/aerospike/backup-go"
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
	client backup.AerospikeClient,
	timestamp time.Time,
) {
	infos := getClusterConfiguration(client)
	if len(infos) == 0 {
		w.logger.Warn("Could not read aerospike configuration")
		return
	}

	for i, info := range infos {
		confFilePath := getConfigurationPath(w.routineName, w.policy, timestamp, i)
		err := storage.WriteFile(ctx, w.storage, confFilePath, []byte(info))
		if err != nil {
			w.logger.Error("Failed to write cluster configuration backup",
				slog.Any("err", err))
		}
	}
}
