package handlers

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/stretchr/testify/mock"
)

// MockBackupBackendService is a mock implementation of BackupReaderWriter.
type MockBackupBackendService struct {
	mock.Mock
}

// GetBackups retrieves backup details based on the provided filter.
func (m *MockBackupBackendService) GetBackups(ctx context.Context, filter service.BackupFilter,
) ([]model.BackupDetails, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]model.BackupDetails), args.Error(1)
}

// WriteBackupMetadata stores metadata for a specific backup.
func (m *MockBackupBackendService) WriteBackupMetadata(
	ctx context.Context,
	routineName, path string,
	metadata model.BackupMetadata,
) error {
	args := m.Called(ctx, routineName, path, metadata)
	return args.Error(0)
}

// Delete removes a specific backup folder.
func (m *MockBackupBackendService) Delete(ctx context.Context, routineName *model.BackupRoutine, path string) error {
	args := m.Called(ctx, routineName, path)
	return args.Error(0)
}

// MockConfigApplier is a configurable no-op ConfigApplier; set Err to simulate a failure.
type MockConfigApplier struct {
	Err error
}

func (a *MockConfigApplier) ApplyNewConfig(_ context.Context) error {
	return a.Err
}
