package shared

import (
	"context"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/backup-go"
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
func (r *RestoreMock) RestoreRun(_ context.Context, _ *aerospike.Client, _ *model.RestoreRequestInternal,
) (*backup.RestoreHandler, error) {
	slog.Info("RestoreRun mock call")
	time.Sleep(100 * time.Millisecond)
	return &backup.RestoreHandler{}, nil
}
