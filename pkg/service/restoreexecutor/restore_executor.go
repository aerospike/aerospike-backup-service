package restoreexecutor

import (
	"context"
	"errors"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/backup-go"
)

// Restore represents a restore service.
type Restore interface {
	Run(
		ctx context.Context,
		client *backup.Client,
		restoreRequest *model.RestoreRequest,
	) (RestoreHandler, error)
}

// DefaultRestoreExecutor implements the [Restore] interface.
type DefaultRestoreExecutor struct {
}

// NewRestore returns a new DefaultRestoreExecutor instance.
func NewRestore() *DefaultRestoreExecutor {
	return &DefaultRestoreExecutor{}
}

// Run initiates the restore operation.
// A restore handler is returned to monitor the job status.
func (r *DefaultRestoreExecutor) Run(
	ctx context.Context,
	client *backup.Client,
	request *model.RestoreRequest,
) (RestoreHandler, error) {
	scanHandler, errScan := runScanRestore(ctx, client, request)
	xdrHandler, errXdr := runXDRRestore(ctx, client, request)

	// Case 1: Both succeed
	if errScan == nil && errXdr == nil {
		return NewCombinedRestoreHandler(scanHandler, xdrHandler), nil
	}

	// Case 2: One returns ErrEmptyStorage, return the other
	if errors.Is(errScan, storage.ErrEmptyStorage) && errXdr == nil {
		return xdrHandler, nil
	}
	if errors.Is(errXdr, storage.ErrEmptyStorage) && errScan == nil {
		return scanHandler, nil
	}

	// Case 3: Both return ErrEmptyStorage
	if errors.Is(errScan, storage.ErrEmptyStorage) && errors.Is(errXdr, storage.ErrEmptyStorage) {
		return nil, storage.ErrEmptyStorage
	}

	// Case 4: One or both have real errors → return joined errors
	return nil, errors.Join(errScan, errXdr)
}
