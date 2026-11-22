package serverbackup

import (
	"context"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

// configRetriever is a placeholder. It finds no cluster configuration: the files the nodes
// upload with a backup are not read.
type configRetriever struct{}

var _ service.ConfigRetriever = configRetriever{}

// NewConfigRetriever returns the ConfigRetriever of the server backup mode. Every retrieval fails
// with [model.ErrNotFound].
func NewConfigRetriever() service.ConfigRetriever {
	return configRetriever{}
}

func (configRetriever) RetrieveConfiguration(context.Context, *model.BackupRoutine, time.Time) ([]byte, error) {
	return nil, fmt.Errorf("the cluster configuration of a server backup cannot be retrieved: %w", model.ErrNotFound)
}
