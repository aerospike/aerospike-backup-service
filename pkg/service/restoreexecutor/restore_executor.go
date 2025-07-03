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
	ops := []func() (RestoreHandler, error){
		func() (RestoreHandler, error) { return runScanRestore(ctx, client, request) },
		func() (RestoreHandler, error) { return runXDRRestore(ctx, client, request) },
	}

	var (
		handlers = make([]RestoreHandler, 0, len(ops))
		errs     error
	)

	// Error handling logic:
	// We treat [storage.ErrEmptyStorage] as a non-fatal.
	// Any other error causes the whole operation to fail immediately.
	// If at least one restore starts successfully, a CombinedRestoreHandler is returned.
	// Otherwise, a joined [storage.ErrEmptyStorage] is returned.
	for _, op := range ops {
		handler, err := op()
		if err != nil {
			if !errors.Is(err, storage.ErrEmptyStorage) {
				return nil, err
			}

			errs = errors.Join(errs, err)
			continue
		}

		handlers = append(handlers, handler)
	}

	if len(handlers) > 0 {
		return NewCombinedRestoreHandler(handlers...), nil
	}

	return nil, errs
}
