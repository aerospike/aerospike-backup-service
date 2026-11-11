package serverbackup

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
)

// retentionManager is a placeholder. It deletes nothing, so server backups are kept until
// retention is written for the layout the server gives them.
type retentionManager struct{}

var _ service.BackupRetentionManager = retentionManager{}

// NewRetentionManager returns the BackupRetentionManager of the server backup mode.
func NewRetentionManager() service.BackupRetentionManager {
	return retentionManager{}
}

func (retentionManager) ApplyRetention(context.Context, *model.BackupRoutine) error {
	return nil
}
