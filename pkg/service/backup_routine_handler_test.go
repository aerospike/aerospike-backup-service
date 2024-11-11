package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations
type mockBackupService struct {
	mock.Mock
}

func (m *mockBackupService) BackupRun(
	ctx context.Context,
	backupRoutine *model.BackupRoutine,
	backupPolicy *model.BackupPolicy,
	client *backup.Client,
	storage model.Storage,
	secretAgent *model.SecretAgent,
	timebounds model.TimeBounds,
	namespace string,
	path string,
) (BackupHandler, error) {
	args := m.Called(ctx, backupRoutine, backupPolicy, client, storage, secretAgent, timebounds, namespace, path)
	return args.Get(0).(BackupHandler), args.Error(1)
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

type mockMetadataWriter struct {
	mock.Mock
}

func (m *mockMetadataWriter) WriteBackupMetadata(
	ctx context.Context, path string, metadata model.BackupMetadata,
) error {
	args := m.Called(ctx, path, metadata)
	return args.Error(0)
}

func (m *mockMetadataWriter) ReadState() *model.BackupState {
	args := m.Called()
	return args.Get(0).(*model.BackupState)
}

type mockClusterConfigWriter struct {
	mock.Mock
}

func (m *mockClusterConfigWriter) Write(ctx context.Context, client backup.AerospikeClient, timestamp time.Time) {
	m.Called(ctx, client, timestamp)
}

func setupTestHandler(
	backupService *mockBackupService,
	clientManager *mockClientManager,
	metadataWriter *mockMetadataWriter,
	configWriter *mockClusterConfigWriter,
) *BackupRoutineHandler {
	return &BackupRoutineHandler{
		namespaces:          []string{"ns1", "ns2"},
		backupService:       backupService,
		clientManager:       clientManager,
		metadataWriter:      metadataWriter,
		clusterConfigWriter: configWriter,
		backupRoutine: &model.BackupRoutine{
			SourceCluster: &model.AerospikeCluster{},
		},
		backupFullPolicy:   &model.BackupPolicy{},
		fullBackupHandlers: make(map[string]BackupHandler),
		incrBackupHandlers: make(map[string]BackupHandler),
		state:              &model.BackupState{},
		storage:            &model.LocalStorage{Path: "/tmp"},
		logger:             slog.Default(),
	}
}

func TestRunFullBackupInternal_SkipIfInProgress(t *testing.T) {
	backupService := new(mockBackupService)
	clientManager := new(mockClientManager)
	metadataWriter := new(mockMetadataWriter)
	configWriter := new(mockClusterConfigWriter)

	handler := setupTestHandler(backupService, clientManager, metadataWriter, configWriter)

	// Simulate backup in progress
	handler.fullBackupHandlers["test"] = &mockBackupHandler{}

	err := handler.runFullBackupInternal(context.Background(), time.Now())

	assert.NoError(t, err, "Should not return error when skipping backup")
}

func TestRunFullBackupInternal_ClientError(t *testing.T) {
	backupService := new(mockBackupService)
	clientManager := new(mockClientManager)
	metadataWriter := new(mockMetadataWriter)
	configWriter := new(mockClusterConfigWriter)

	handler := setupTestHandler(backupService, clientManager, metadataWriter, configWriter)

	expectedErr := errors.New("client error")
	clientManager.On("GetClient", mock.Anything).Return(nil, expectedErr)

	err := handler.runFullBackupInternal(context.Background(), time.Now())

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	clientManager.AssertExpectations(t)
}

func TestRunFullBackupInternal_Success(t *testing.T) {
	backupService := new(mockBackupService)
	clientManager := new(mockClientManager)
	metadataWriter := new(mockMetadataWriter)
	configWriter := new(mockClusterConfigWriter)

	handler := setupTestHandler(backupService, clientManager, metadataWriter, configWriter)

	// Setup mocks
	client := &backup.Client{}
	clientManager.On("GetClient", mock.Anything).Return(client, nil)
	clientManager.On("Close", client).Return()

	backupHandler := new(mockBackupHandler)
	backupHandler.On("Wait", mock.Anything).Return(nil)
	backupHandler.On("GetStats").Return(&models.BackupStats{})

	// Expect backup run for each namespace
	backupService.On("BackupRun",
		mock.Anything, // ctx
		mock.Anything, // backupRoutine
		mock.Anything, // backupPolicy
		mock.Anything, // client
		mock.Anything, // storage
		mock.Anything, // secretAgent
		mock.Anything, // timebounds
		"ns1",         // namespace
		mock.Anything, // path
	).Return(backupHandler, nil).Once()

	backupService.On("BackupRun",
		mock.Anything, // ctx
		mock.Anything, // backupRoutine
		mock.Anything, // backupPolicy
		mock.Anything, // client
		mock.Anything, // storage
		mock.Anything, // secretAgent
		mock.Anything, // timebounds
		"ns2",         // namespace
		mock.Anything, // path
	).Return(backupHandler, nil).Once()

	metadataWriter.On("WriteBackupMetadata",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil).Times(2) // Once for each namespace

	configWriter.On("Write",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return()

	err := handler.runFullBackupInternal(context.Background(), time.Now())

	assert.NoError(t, err)
	clientManager.AssertExpectations(t)
	backupService.AssertExpectations(t)
	metadataWriter.AssertExpectations(t)
	configWriter.AssertExpectations(t)
}

func TestRunFullBackupInternal_BackupError(t *testing.T) {
	backupService := new(mockBackupService)
	clientManager := new(mockClientManager)
	metadataWriter := new(mockMetadataWriter)
	configWriter := new(mockClusterConfigWriter)

	handler := setupTestHandler(backupService, clientManager, metadataWriter, configWriter)

	// Setup mocks
	client := &backup.Client{}
	clientManager.On("GetClient", mock.Anything).Return(client, nil)
	clientManager.On("Close", client).Return()

	expectedErr := errors.New("backup error")
	backupService.On("BackupRun",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(&mockBackupHandler{}, expectedErr)

	err := handler.runFullBackupInternal(context.Background(), time.Now())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), expectedErr.Error())
	clientManager.AssertExpectations(t)
	backupService.AssertExpectations(t)
}

func TestRunFullBackupInternal_WaitError(t *testing.T) {
	backupService := new(mockBackupService)
	clientManager := new(mockClientManager)
	metadataWriter := new(mockMetadataWriter)
	configWriter := new(mockClusterConfigWriter)

	handler := setupTestHandler(backupService, clientManager, metadataWriter, configWriter)

	// Setup mocks
	client := &backup.Client{}
	clientManager.On("GetClient", mock.Anything).Return(client, nil)
	clientManager.On("Close", client).Return()

	backupHandler := new(mockBackupHandler)
	expectedErr := errors.New("wait error")
	backupHandler.On("Wait", mock.Anything).Return(expectedErr)

	backupService.On("BackupRun",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(backupHandler, nil)

	err := handler.runFullBackupInternal(context.Background(), time.Now())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), expectedErr.Error())
	clientManager.AssertExpectations(t)
	backupService.AssertExpectations(t)
	backupHandler.AssertExpectations(t)
}
