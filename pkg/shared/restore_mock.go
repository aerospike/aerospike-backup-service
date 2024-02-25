//go:build ci

package shared

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/backup/pkg/model"
)

// RestoreShared mocks the Restore interface.
// Used in CI workflows to skip building the C shared libraries.
type RestoreShared struct {
}

var _ Restore = (*RestoreShared)(nil)

// NewRestore returns a new RestoreShared instance.
func NewRestore() *RestoreShared {
	return &RestoreShared{}
}

// RestoreRun mocks the interface method.
func (r *RestoreShared) RestoreRun(restoreRequest *model.RestoreRequestInternal) (*model.RestoreResult, error) {
	if restoreRequest.DestinationCuster == nil {
		return nil, fmt.Errorf("RestoreRun mock call without DestinationCuster provided, will fail")
	}
	slog.Info("RestoreRun mock call")
	time.Sleep(100 * time.Millisecond)
	result := model.NewRestoreResult()
	result.TotalRecords = 1
	return result, nil
}
