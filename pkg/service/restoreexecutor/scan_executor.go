package restoreexecutor

import (
	"context"
	"errors"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go/io/storage/common"
)

// ScanRestoreExecutor runs client-side scan and XDR restores from object storage.
type ScanRestoreExecutor struct {
	operations storageReader
}

// NewScanRestore creates an executor that restores scan- and XDR-format backup data.
func NewScanRestore(operations storageReader) *ScanRestoreExecutor {
	return &ScanRestoreExecutor{operations: operations}
}

// NewRestore is an alias for [NewScanRestore].
func NewRestore(operations storageReader) *ScanRestoreExecutor {
	return NewScanRestore(operations)
}

// Run initiates the restore operation.
// A restore handler is returned to monitor the job status.
func (r *ScanRestoreExecutor) Run(
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
