package service

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// Backup represents a backup service.
type Backup interface {
	BackupRun(
		ctx context.Context,
		backupRoutine *model.BackupRoutine,
		backupPolicy *model.BackupPolicy,
		client *backup.Client,
		storage *model.Storage,
		secretAgent *model.SecretAgent,
		timebounds model.TimeBounds,
		namespace string,
		path *string,
	) (BackupHandler, error)
}

// RestoreHandler represents a restore handler returned by the backup client.
type RestoreHandler interface {
	// GetStats returns the statistics of the restore job.
	GetStats() *models.RestoreStats
	// Wait waits for the restore job to complete and returns an error if the
	// job failed.
	Wait() error
}

// BackupHandler represents a backup handler returned by the backup client.
type BackupHandler interface {
	// GetStats returns the statistics of the backup job.
	GetStats() *models.BackupStats
	// Wait waits for the backup job to complete and returns an error if the
	// job failed.
	Wait() error
}

// Restore represents a restore service.
type Restore interface {
	RestoreRun(
		ctx context.Context,
		client *backup.Client,
		restoreRequest *model.RestoreRequestInternal,
	) (RestoreHandler, error)
}
