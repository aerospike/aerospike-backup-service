package shared

import (
	"context"
	"time"

	"github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup/pkg/model"
)

// BackupOptions provides additional properties for running a backup.
type BackupOptions struct {
	ModBefore *time.Time
	ModAfter  *time.Time
}

// Backup represents a backup service.
type Backup interface {
	BackupRun(ctx context.Context,
		backupRoutine *model.BackupRoutine,
		backupPolicy *model.BackupPolicy,
		client *aerospike.Client,
		storage *model.Storage,
		secretAgent *model.SecretAgent,
		opts BackupOptions,
		namespace *string,
		path *string,
	) (*backup.BackupHandler, error)
}

// Restore represents a restore service.
type Restore interface {
	RestoreRun(ctx context.Context, restoreRequest *model.RestoreRequestInternal) (*model.RestoreResult, error)
}
