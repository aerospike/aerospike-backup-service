package handlers

import (
	"context"
	"time"

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

// MockConfigRetriever is a mock implementation of service.ConfigRetriever.
type MockConfigRetriever struct {
	mock.Mock
}

// RetrieveConfiguration returns backed up Aerospike configuration.
func (m *MockConfigRetriever) RetrieveConfiguration(
	ctx context.Context, routine *model.BackupRoutine, timestamp time.Time,
) ([]byte, error) {
	args := m.Called(ctx, routine, timestamp)
	buf, _ := args.Get(0).([]byte)
	return buf, args.Error(1)
}

type configurationManagerMock struct{}

func (mock configurationManagerMock) Read(_ context.Context) (*model.Config, error) {
	return nil, nil
}

func (mock configurationManagerMock) Write(_ context.Context, config *model.Config) error {
	if config == nil {
		return nil
	}
	return nil
}

// MockConfigApplier is a configurable no-op ConfigApplier; set Err to simulate a failure.
type MockConfigApplier struct {
	Err error
}

func (a *MockConfigApplier) ApplyNewConfig(_ context.Context) error {
	return a.Err
}

// MockConfigurationManager is a configurable mock implementation of configuration.Manager.
type MockConfigurationManager struct {
	mock.Mock
}

// Read reads the configuration from the source.
func (m *MockConfigurationManager) Read(ctx context.Context) (*model.Config, error) {
	args := m.Called(ctx)
	cfg, _ := args.Get(0).(*model.Config)
	return cfg, args.Error(1)
}

// Write writes the configuration to the source.
func (m *MockConfigurationManager) Write(ctx context.Context, config *model.Config) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}
