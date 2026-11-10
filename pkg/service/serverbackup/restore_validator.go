package serverbackup

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
)

// RestoreValidator checks whether a server restore can run, before the server is asked to run it.
type RestoreValidator interface {
	// ValidatePath validates the restore of the one backup a request names by its key.
	ValidatePath(
		ctx context.Context,
		request *model.RestoreRequest,
		infoGetter backup.InfoGetter,
		backups []model.BackupDetails,
	) error
	// ValidateTimestamp validates a restore to a point in time over the backups it would apply.
	ValidateTimestamp(
		ctx context.Context,
		request *model.RestoreTimestampRequest,
		infoGetter backup.InfoGetter,
		backups []model.BackupDetails,
	) error
}

type nopRestoreValidator struct{}

var _ RestoreValidator = nopRestoreValidator{}

// NopRestoreValidator returns a RestoreValidator that accepts every restore. The server refuses
// a restore it cannot run, and that refusal fails the restore job.
func NopRestoreValidator() RestoreValidator {
	return nopRestoreValidator{}
}

func (nopRestoreValidator) ValidatePath(
	context.Context, *model.RestoreRequest, backup.InfoGetter, []model.BackupDetails,
) error {
	return nil
}

func (nopRestoreValidator) ValidateTimestamp(
	context.Context, *model.RestoreTimestampRequest, backup.InfoGetter, []model.BackupDetails,
) error {
	return nil
}
