package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
)

// RestoreMock mocks the Restore interface.
// Used in CI workflows to skip building the C shared libraries.
type RestoreMock struct {
	restoreWaitWg *sync.WaitGroup
}

// NewRestoreMock returns a new RestoreMock instance.
func NewRestoreMock(wg *sync.WaitGroup) *RestoreMock {
	return &RestoreMock{
		restoreWaitWg: wg,
	}
}

// MockRestoreHandler is a mock implementation of the RestoreHandler interface.
type MockRestoreHandler struct {
	restoreWaitWg *sync.WaitGroup
}

func (m *MockRestoreHandler) GetStats() *models.RestoreStats {
	stats := models.NewRestoreStats()
	stats.ReadRecords.Add(1)
	return stats
}

func (m *MockRestoreHandler) Wait(ctx context.Context) error {
	if m.restoreWaitWg != nil {
		m.restoreWaitWg.Done()
	}

	select {
	case <-time.After(100 * time.Millisecond):
		// Simulate work completion after 100ms
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Run mocks the interface method.
func (r *RestoreMock) Run(_ context.Context, _ *backup.Client,
	_ *model.RestoreRequest) (RestoreHandler, error) {
	slog.Info("RestoreRun mock call")
	return &MockRestoreHandler{
		restoreWaitWg: r.restoreWaitWg,
	}, nil
}
