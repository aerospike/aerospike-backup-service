package service

import (
	"context"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

type nopRetentionManager struct{}

var _ RetentionManager = (*nopRetentionManager)(nil)

// NewNopRetentionManager returns a retention manager that performs no cleanup.
func NewNopRetentionManager() RetentionManager {
	return &nopRetentionManager{}
}

func (nopRetentionManager) deleteOldBackups(context.Context, *model.BackupRoutine) error {
	return nil
}

type nopClusterConfigWriter struct{}

var _ ClusterConfigWriter = (*nopClusterConfigWriter)(nil)

// NewNopClusterConfigWriter returns a cluster config writer that performs no writes.
func NewNopClusterConfigWriter() ClusterConfigWriter {
	return &nopClusterConfigWriter{}
}

func (nopClusterConfigWriter) Write(context.Context, *model.BackupRoutine, time.Time) error {
	return nil
}
