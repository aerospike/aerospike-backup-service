package serverbackup

import (
	"context"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

// clusterConfigWriter writes nothing: in server mode each node uploads its own configuration
// file with the backup.
type clusterConfigWriter struct{}

var _ service.ClusterConfigWriter = clusterConfigWriter{}

// NewClusterConfigWriter returns the ClusterConfigWriter of the server backup mode.
func NewClusterConfigWriter() service.ClusterConfigWriter {
	return clusterConfigWriter{}
}

func (clusterConfigWriter) Write(context.Context, *model.BackupRoutine, time.Time) error {
	return nil
}
