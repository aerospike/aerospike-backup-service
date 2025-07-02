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

	// Error handling logic:
	// We treat [storage.ErrEmptyStorage] as a non-fatal.
	// Any other error causes the whole operation to fail immediately.
	// If at least one restore starts successfully, a CombinedRestoreHandler is returned.
	// Otherwise, [storage.ErrEmptyStorage] is returned.
	var successHandlers []RestoreHandler

	// Scan result
	switch {
	case errScan == nil:
		successHandlers = append(successHandlers, scanHandler)
	case !errors.Is(errScan, storage.ErrEmptyStorage):
		return nil, errScan
	}

	// XDR result
	switch {
	case errXdr == nil:
		successHandlers = append(successHandlers, xdrHandler)
	case !errors.Is(errXdr, storage.ErrEmptyStorage):
		return nil, errXdr
	}

	if len(successHandlers) == 0 {
		return nil, storage.ErrEmptyStorage
	}

	return NewCombinedRestoreHandler(successHandlers...), nil
}
