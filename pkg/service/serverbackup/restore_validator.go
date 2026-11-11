package serverbackup

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
)

// RestoreValidator checks whether a server restore can run, before the server is asked to run it.
// The checks a server restore needs will be written against this interface.
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

// restoreValidator is a placeholder. It accepts every restore. The checks of the scan mode do
// not carry over, because most of them are about options a server restore does not have, such
// as compression. A server restore needs at least to be refused while a backup of the same
// namespace runs.
type restoreValidator struct{}

var _ RestoreValidator = restoreValidator{}

// NewRestoreValidator returns the RestoreValidator of the server backup mode. The server refuses
// a restore it cannot run, and that refusal fails the restore job.
func NewRestoreValidator() RestoreValidator {
	return restoreValidator{}
}

func (restoreValidator) ValidatePath(
	context.Context, *model.RestoreRequest, backup.InfoGetter, []model.BackupDetails,
) error {
	return nil
}

func (restoreValidator) ValidateTimestamp(
	context.Context, *model.RestoreTimestampRequest, backup.InfoGetter, []model.BackupDetails,
) error {
	return nil
}
