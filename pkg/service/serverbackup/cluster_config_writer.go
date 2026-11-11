package serverbackup

import (
	"context"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

// clusterConfigWriter is a placeholder. It writes nothing, so a server backup holds no cluster
// configuration.
type clusterConfigWriter struct{}

var _ service.ClusterConfigWriter = clusterConfigWriter{}

// NewClusterConfigWriter returns the ClusterConfigWriter of the server backup mode.
func NewClusterConfigWriter() service.ClusterConfigWriter {
	return clusterConfigWriter{}
}

func (clusterConfigWriter) Write(context.Context, *model.BackupRoutine, time.Time) error {
	return nil
}
