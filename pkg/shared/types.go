package shared

import (
	"context"

	"github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup/pkg/model"
)

// Backup represents a backup service.
type Backup interface {
	BackupRun(ctx context.Context,
		backupRoutine *model.BackupRoutine,
		backupPolicy *model.BackupPolicy,
		client *aerospike.Client,
		storage *model.Storage,
		secretAgent *model.SecretAgent,
		timebounds model.TimeBounds,
		namespace *string,
		path *string,
	) (*backup.BackupHandler, error)
}

// Restore represents a restore service.
type Restore interface {
	RestoreRun(ctx context.Context, client *aerospike.Client, restoreRequest *model.RestoreRequestInternal,
	) (*model.RestoreResult, error)
}
