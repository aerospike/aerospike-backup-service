//go:build ci

package shared

import (
	"log/slog"

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
func (r *RestoreShared) RestoreRun(restoreRequest *model.RestoreRequestInternal) *model.RestoreResult {
	slog.Info("RestoreRun mock call")
	result := model.NewRestoreResult()
	result.Number = 1
	return result
}
