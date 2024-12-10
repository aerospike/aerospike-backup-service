package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// RestoreMock mocks the Restore interface.
// Used in CI workflows to skip building the C shared libraries.
type RestoreMock struct {
}

// NewRestoreMock returns a new RestoreMock instance.
func NewRestoreMock() *RestoreMock {
	return &RestoreMock{}
}

// MockRestoreHandler is a mock implementation of the RestoreHandler interface.
type MockRestoreHandler struct {
	wasCancelled bool
}

func (m *MockRestoreHandler) GetStats() *models.RestoreStats {
	stats := models.RestoreStats{}
	stats.ReadRecords.Add(1)
	return &stats
}

func (m *MockRestoreHandler) Wait(ctx context.Context) error {
	select {
	case <-time.After(100 * time.Millisecond):
		// Simulate work completion after 100ms
		return nil
	case <-ctx.Done():
		m.wasCancelled = true
		return ctx.Err()
	}
}

// Run mocks the interface method.
func (r *RestoreMock) Run(_ context.Context, _ *backup.Client,
	_ *model.RestoreRequest) (RestoreHandler, error) {
	slog.Info("RestoreRun mock call")
	return &MockRestoreHandler{}, nil
}
