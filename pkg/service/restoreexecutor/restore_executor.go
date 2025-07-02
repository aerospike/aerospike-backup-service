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

	var (
		successHandlers []RestoreHandler
		emptyErr        error
	)

	// Logic:
	// We treat ErrEmptyStorage as a non-fatal.
	// A real error from any restore (not ErrEmptyStorage) fails the whole operation immediately.
	// If at least one restore starts successfully, we return a CombinedRestoreHandler.
	// If all restores return ErrEmptyStorage, we return the first such error.

	// Scan result
	switch {
	case errScan == nil:
		successHandlers = append(successHandlers, scanHandler)
	case errors.Is(errScan, storage.ErrEmptyStorage):
		emptyErr = errScan
	default:
		return nil, errScan
	}

	// XDR result
	switch {
	case errXdr == nil:
		successHandlers = append(successHandlers, xdrHandler)
	case errors.Is(errXdr, storage.ErrEmptyStorage):
		if emptyErr == nil {
			emptyErr = errXdr
		}
	default:
		return nil, errXdr
	}

	if len(successHandlers) == 0 {
		return nil, emptyErr
	}

	return NewCombinedRestoreHandler(successHandlers...), nil
}
