package service

import (
	"context"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/backup-go"
	"github.com/stretchr/testify/mock"
)

type mockBackupExecutor struct {
	mock.Mock
}

func (m *mockBackupExecutor) Run(
	ctx context.Context,
	client aerospike.Backuper,
	routine *model.BackupRoutine,
	timeBounds model.TimeBounds,
	namespace string,
	path string,
) (backupexecutor.BackupHandler, error) {
	args := m.Called(ctx, client, routine, timeBounds, namespace, path)
	return args.Get(0).(backupexecutor.BackupHandler), args.Error(1)
}

type mockClientManager struct {
	mock.Mock
}

func (m *mockClientManager) GetClient(cluster *model.AerospikeCluster) (aerospike.Client, error) {
	args := m.Called(cluster)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*backup.Client), args.Error(1)
}

func (m *mockClientManager) Close(client aerospike.Client) {
	m.Called(client)
}

type mockClusterConfigWriter struct {
	mock.Mock
}

func (m *mockClusterConfigWriter) Write(ctx context.Context, routineName string, timestamp time.Time) error {
	args := m.Called(ctx, routineName, timestamp)
	return args.Error(0)
}

type mockRetentionManager struct {
	mock.Mock
}

func (m *mockRetentionManager) deleteOldBackups(ctx context.Context, routineName string) error {
	args := m.Called(ctx, routineName)
	return args.Error(0)
}

// MockBackupBackendService is a mock implementation of BackupReaderWriter.
type MockBackupBackendService struct {
	mock.Mock
}

// GetBackups retrieves backup details based on the provided filter.
func (m *MockBackupBackendService) GetBackups(ctx context.Context, filter BackupFilter) ([]model.BackupDetails, error) {
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
func (m *MockBackupBackendService) Delete(ctx context.Context, routineName, path string) error {
	args := m.Called(ctx, routineName, path)
	return args.Error(0)
}
