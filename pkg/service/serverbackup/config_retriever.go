package serverbackup

import (
	"context"
	"fmt"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

type configRetriever struct{}

var _ service.ConfigRetriever = configRetriever{}

// NewConfigRetriever returns the ConfigRetriever of the server backup mode. A server backup holds
// no cluster configuration, so every retrieval fails with [model.ErrNotFound].
func NewConfigRetriever() service.ConfigRetriever {
	return configRetriever{}
}

func (configRetriever) RetrieveConfiguration(context.Context, *model.BackupRoutine, time.Time) ([]byte, error) {
	return nil, fmt.Errorf("a server backup holds no cluster configuration: %w", model.ErrNotFound)
}
