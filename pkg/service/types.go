package service

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/pkg/dto"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// Backup represents a backup service.
type Backup interface {
	BackupRun(
		ctx context.Context,
		backupRoutine *dto.BackupRoutine,
		backupPolicy *dto.BackupPolicy,
		client *backup.Client,
		storage *dto.Storage,
		secretAgent *dto.SecretAgent,
		timebounds dto.TimeBounds,
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
		restoreRequest *dto.RestoreRequestInternal,
	) (RestoreHandler, error)
}
