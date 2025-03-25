package service

import (
	"context"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/mock"
	"time"
)

type mockBackupExecutor struct {
	mock.Mock
}

func (m *mockBackupExecutor) Run(
	ctx context.Context,
	client *backup.Client,
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

func (m *mockClientManager) GetClient(cluster *model.AerospikeCluster) (*backup.Client, error) {
	args := m.Called(cluster)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*backup.Client), args.Error(1)
}

func (m *mockClientManager) Close(client *backup.Client) {
	m.Called(client)
}

func clientManagerMock() (*mockClientManager, *backup.Client) {
	client := &backup.Client{}
	clientManager := new(mockClientManager)
	clientManager.On("GetClient", mock.Anything).Return(client, nil)
	clientManager.On("Close", client).Return()
	return clientManager, client
}

type mockBackupHandler struct {
	mock.Mock
}

func (m *mockBackupHandler) GetStats() *models.BackupStats {
	args := m.Called()
	return args.Get(0).(*models.BackupStats)
}

func (m *mockBackupHandler) Wait(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockBackupHandler) Cancel() {}

type mockClusterConfigWriter struct {
	mock.Mock
}

func (m *mockClusterConfigWriter) Write(ctx context.Context, routineName string, timestamp time.Time) {
	m.Called(ctx, routineName, timestamp)
}

type mockRetentionManager struct {
	mock.Mock
}

func (m *mockRetentionManager) deleteOldBackups(ctx context.Context, routineName string) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockRunningBackupsRegistry is a mock implementation of RunningBackupsRegistry
type MockRunningBackupsRegistry struct {
	mock.Mock
}

// register adds a new backup handler for a specific routine and job type
func (m *MockRunningBackupsRegistry) register(routineName string, jt jobType, handler CancelableBackupHandler) {
	m.Called(routineName, jt, handler)
}

// remove deletes a backup from the registry
func (m *MockRunningBackupsRegistry) remove(routineName string, jt jobType) {
	m.Called(routineName, jt)
}

// unregister removes a backup from the registry and updates the last success timestamp
func (m *MockRunningBackupsRegistry) unregister(routineName string, jt jobType, timestamp time.Time) {
	m.Called(routineName, jt, timestamp)
}

// GetRoutineState returns the current backup statistics for a routine
func (m *MockRunningBackupsRegistry) GetRoutineState(routineName string) *model.RoutineState {
	args := m.Called(routineName)
	return args.Get(0).(*model.RoutineState)
}

// GetRunningState returns statistics for all current backups
func (m *MockRunningBackupsRegistry) GetRunningState() map[string]*model.RoutineState {
	args := m.Called()
	return args.Get(0).(map[string]*model.RoutineState)
}

// Cancel stops all ongoing backups for a specific routine
func (m *MockRunningBackupsRegistry) Cancel(routineName string) {
	m.Called(routineName)
}

// SynchroniseBackupHistory updates the backup registry with the most recent backup timestamps
func (m *MockRunningBackupsRegistry) SynchroniseBackupHistory() {
	m.Called()
}

func (m *MockRunningBackupsRegistry) ExpectRegister(routineName string, jt jobType, handler CancelableBackupHandler) *mock.Call {
	return m.On("register", routineName, jt, handler)
}

func (m *MockRunningBackupsRegistry) ExpectRemove(routineName string, jt jobType) *mock.Call {
	return m.On("remove", routineName, jt)
}

func (m *MockRunningBackupsRegistry) ExpectUnregister(routineName string, jt jobType, timestamp time.Time) *mock.Call {
	return m.On("unregister", routineName, jt, timestamp)
}

func (m *MockRunningBackupsRegistry) ExpectGetRoutineState(routineName string, state *model.RoutineState) *mock.Call {
	return m.On("GetRoutineState", routineName).Return(state)
}

func (m *MockRunningBackupsRegistry) ExpectGetRunningState(state map[string]*model.RoutineState) *mock.Call {
	return m.On("GetRunningState").Return(state)
}

func (m *MockRunningBackupsRegistry) ExpectCancel(routineName string) *mock.Call {
	return m.On("Cancel", routineName)
}

func (m *MockRunningBackupsRegistry) ExpectSynchroniseBackupHistory() *mock.Call {
	return m.On("SynchroniseBackupHistory")
}

// MockBackupBackendService is a mock implementation of BackupBackendService
type MockBackupBackendService struct {
	mock.Mock
}

// GetBackups retrieves backup details based on the provided filter
func (m *MockBackupBackendService) GetBackups(ctx context.Context, filter BackupFilter) ([]model.BackupDetails, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]model.BackupDetails), args.Error(1)
}

// WriteBackupMetadata stores metadata for a specific backup
func (m *MockBackupBackendService) WriteBackupMetadata(ctx context.Context, routineName, path string, metadata model.BackupMetadata) error {
	args := m.Called(ctx, routineName, path, metadata)
	return args.Error(0)
}

// Delete removes a specific backup folder
func (m *MockBackupBackendService) Delete(ctx context.Context, routineName, path string) error {
	args := m.Called(ctx, routineName, path)
	return args.Error(0)
}

// Helper methods for easy expectation setup

func (m *MockBackupBackendService) ExpectGetBackups(ctx context.Context, filter BackupFilter, details []model.BackupDetails, err error) *mock.Call {
	return m.On("GetBackups", ctx, filter).Return(details, err)
}

func (m *MockBackupBackendService) ExpectWriteBackupMetadata(ctx context.Context, routineName, path string, metadata model.BackupMetadata, err error) *mock.Call {
	return m.On("WriteBackupMetadata", ctx, routineName, path, metadata).Return(err)
}

func (m *MockBackupBackendService) ExpectDelete(ctx context.Context, routineName, path string, err error) *mock.Call {
	return m.On("Delete", ctx, routineName, path).Return(err)
}
