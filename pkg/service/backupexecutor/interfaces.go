package backupexecutor

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// BackupHandler interface defines the contract for backup operation results.
type BackupHandler interface {
	GetStats() *models.BackupStats
	Wait(context.Context) error
}

// BackupExecutor defines the interface for running backups.
type BackupExecutor interface {
	Run(
		ctx context.Context,
		client *backup.Client,
		routine *model.BackupRoutine,
		timeBounds model.TimeBounds,
		namespace string,
		path string,
	) (BackupHandler, error)
}
