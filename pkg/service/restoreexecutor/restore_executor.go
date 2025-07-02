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
	restoreTasks := []func() (RestoreHandler, error){
		func() (RestoreHandler, error) { return runScanRestore(ctx, client, request) },
		func() (RestoreHandler, error) { return runXDRRestore(ctx, client, request) },
	}

	var (
		successHandlers []RestoreHandler
		firstEmptyErr   error
	)

	for _, task := range restoreTasks {
		handler, err := task()

		if err == nil {
			successHandlers = append(successHandlers, handler)
			continue
		}

		// A real error fails the whole operation immediately.
		if !errors.Is(err, storage.ErrEmptyStorage) {
			return nil, err
		}

		// We treat ErrEmptyStorage as non-fatal but record the first one.
		if firstEmptyErr == nil {
			firstEmptyErr = err
		}
	}

	// If at least one restore started, return a combined handler.
	if len(successHandlers) > 0 {
		return NewCombinedRestoreHandler(successHandlers...), nil
	}

	// Otherwise, all restores returned ErrEmptyStorage, so we return that error.
	return nil, firstEmptyErr
}
