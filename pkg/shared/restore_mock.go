package shared

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/backup/pkg/model"
)

// RestoreMock mocks the Restore interface.
// Used in CI workflows to skip building the C shared libraries.
type RestoreMock struct {
}

var _ Restore = (*RestoreMock)(nil)

// NewRestoreMock returns a new RestoreMock instance.
func NewRestoreMock() *RestoreMock {
	return &RestoreMock{}
}

// RestoreRun mocks the interface method.
func (r *RestoreMock) RestoreRun(_ context.Context, restoreRequest *model.RestoreRequestInternal,
) (*model.RestoreResult, error) {
	if restoreRequest.DestinationCuster == nil {
		return nil, fmt.Errorf("RestoreRun mock call without DestinationCuster provided, will fail")
	}
	slog.Info("RestoreRun mock call")
	time.Sleep(100 * time.Millisecond)
	result := model.NewRestoreResult()
	result.TotalRecords = 1
	return result, nil
}
