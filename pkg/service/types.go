package service

import (
	"context"

	"github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup/pkg/model"
)

// Backup represents a backup service.
type Backup interface {
	BackupRun(
		ctx context.Context,
		backupRoutine *model.BackupRoutine,
		backupPolicy *model.BackupPolicy,
		client *aerospike.Client,
		storage *model.Storage,
		secretAgent *model.SecretAgent,
		timebounds model.TimeBounds,
		namespace string,
		path *string,
	) (BackupHandler, error)
}

type RestoreHandler interface {
	GetStats() *models.RestoreStats
	Wait() error
}

type BackupHandler interface {
	GetStats() *models.BackupStats
	Wait() error
}

// Restore represents a restore service.
type Restore interface {
	RestoreRun(
		ctx context.Context,
		client *aerospike.Client,
		restoreRequest *model.RestoreRequestInternal,
	) (RestoreHandler, error)
}
