package service

import (
	"context"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

type nopRetentionManager struct{}

var _ BackupRetentionManager = nopRetentionManager{}

// NopRetentionManager returns a BackupRetentionManager that deletes nothing.
func NopRetentionManager() BackupRetentionManager {
	return nopRetentionManager{}
}

func (nopRetentionManager) ApplyRetention(context.Context, *model.BackupRoutine) error {
	return nil
}

type nopClusterConfigWriter struct{}

var _ ClusterConfigWriter = nopClusterConfigWriter{}

// NopClusterConfigWriter returns a ClusterConfigWriter that writes nothing.
func NopClusterConfigWriter() ClusterConfigWriter {
	return nopClusterConfigWriter{}
}

func (nopClusterConfigWriter) Write(context.Context, *model.BackupRoutine, time.Time) error {
	return nil
}
