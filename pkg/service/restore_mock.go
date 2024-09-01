package service

import (
	"context"
	"github.com/aerospike/aerospike-backup-service/pkg/model"
	"log/slog"

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
}

func (m *MockRestoreHandler) GetStats() *models.RestoreStats {
	stats := models.RestoreStats{}
	stats.ReadRecords.Add(1)
	return &stats
}

func (m *MockRestoreHandler) Wait() error {
	return nil
}

// RestoreRun mocks the interface method.
func (r *RestoreMock) RestoreRun(_ context.Context, _ *backup.Client,
	_ *model.RestoreRequestInternal) (RestoreHandler, error) {
	slog.Info("RestoreRun mock call")
	return &MockRestoreHandler{}, nil
}
