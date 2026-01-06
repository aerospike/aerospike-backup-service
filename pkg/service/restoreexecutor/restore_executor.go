package restoreexecutor

import (
	"context"
	"errors"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/io/storage/common"
	"github.com/aerospike/backup-go/io/storage/options"
)

type storageReader interface {
	// CreateDirReader creates a reader for a folder in the specified storage.
	CreateDirReader(ctx context.Context, storage model.Storage, path string, opts ...options.Opt,
	) (backup.StreamingReader, error)
}

// Restore represents a restore service.
type Restore interface {
	// Run executes the restore process and returns a restore handler for monitoring progress.
	Run(
		ctx context.Context,
		client aerospike.Restorer,
		restoreRequest *model.RestoreRequest,
	) (RestoreHandler, error)
}

// DefaultRestoreExecutor implements the [Restore] interface.
type DefaultRestoreExecutor struct {
	operations storageReader
}

// NewRestore returns a new DefaultRestoreExecutor instance.
func NewRestore(operations storageReader) *DefaultRestoreExecutor {
	return &DefaultRestoreExecutor{
		operations: operations,
	}
}

// Run initiates the restore operation.
// A restore handler is returned to monitor the job status.
func (r *DefaultRestoreExecutor) Run(
	ctx context.Context,
	client aerospike.Restorer,
	request *model.RestoreRequest,
) (RestoreHandler, error) {
	ops := []func() (RestoreHandler, error){
		func() (RestoreHandler, error) { return runScanRestore(ctx, client, request, r.operations) },
		func() (RestoreHandler, error) { return runXDRRestore(ctx, client, request, r.operations) },
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
			if !errors.Is(err, common.ErrEmptyStorage) {
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
