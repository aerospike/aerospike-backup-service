package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/mock"
)

// Mock implementations.
type mockBackupService struct {
	mock.Mock
}

func (m *mockBackupService) Run(
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

type mockMetadataWriter struct {
	mock.Mock
}

func (m *mockMetadataWriter) deleteFolder(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *mockMetadataWriter) writeBackupMetadata(
	ctx context.Context, path string, metadata model.BackupMetadata,
) error {
	args := m.Called(ctx, path, metadata)
	return args.Error(0)
}

type mockClusterConfigWriter struct {
	mock.Mock
}

func (m *mockClusterConfigWriter) Write(ctx context.Context, client AerospikeCluster, timestamp time.Time) {
	m.Called(ctx, client, timestamp)
}

type mockRetentionManager struct {
	mock.Mock
}

func (m *mockRetentionManager) deleteOldBackups(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func setupTestHandler(
	backupService *mockBackupService,
	clientManager *mockClientManager,
	metadataWriter *mockMetadataWriter,
	configWriter *mockClusterConfigWriter,
	retentionManager *mockRetentionManager,
) *BackupRoutineOrchestrator {
	return &BackupRoutineOrchestrator{
		routineName:         "routine",
		namespaces:          []string{"ns1", "ns2"},
		backupService:       backupService,
		clientManager:       clientManager,
		clusterConfigWriter: configWriter,
		backupRoutine: &model.BackupRoutine{
			SourceCluster: &model.AerospikeCluster{},
		},
		logger:           slog.Default(),
		retry:            &simpleExecutor{},
		registry:         NewRunningBackupsRegistry(context.Background(), NewBackupBackends()),
		retentionManager: retentionManager,
		runner: NewBackupNamespaceRunner(
			"routine",
			backupService,
			&simpleExecutor{},
			metadataWriter,
			slog.Default(),
		),
	}
}

func TestRunFullBackupInternal_Success(t *testing.T) {
	backupService := new(mockBackupService)
	clientManager := clientManagerMock()
	metadataWriter := new(mockMetadataWriter)
	configWriter := new(mockClusterConfigWriter)
	retentionManager := new(mockRetentionManager)

	handler := setupTestHandler(backupService, clientManager, metadataWriter, configWriter, retentionManager)

	backupHandler := new(mockBackupHandler)
	backupHandler.On("Wait", mock.Anything).Return(nil)
	backupHandler.On("GetStats").Return(models.NewBackupStats())

	// Expect backup run for each namespace
	backupService.On("Run",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		"ns1",
		mock.Anything,
	).Return(backupHandler, nil).Once()

	backupService.On("Run",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		"ns2",
		mock.Anything,
	).Return(backupHandler, nil).Once()

	metadataWriter.On("writeBackupMetadata",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil).Times(2) // Once for each namespace

	configWriter.On("Write",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return()

	retentionManager.On("deleteOldBackups", mock.Anything).Return(nil)

	handler.runFullBackup(context.Background(), time.Now())

	clientManager.AssertExpectations(t)
	backupService.AssertExpectations(t)
	metadataWriter.AssertExpectations(t)
	configWriter.AssertExpectations(t)
	retentionManager.AssertExpectations(t)
}

func TestRunFullBackupInternal_WaitError(t *testing.T) {
	backupService := new(mockBackupService)
	clientManager := clientManagerMock()
	metadataWriter := new(mockMetadataWriter)
	configWriter := new(mockClusterConfigWriter)
	retentionManager := new(mockRetentionManager)

	handler := setupTestHandler(backupService, clientManager, metadataWriter, configWriter, retentionManager)

	backupHandler := new(mockBackupHandler)
	expectedErr := errors.New("wait error")
	backupHandler.On("Wait", mock.Anything).Return(expectedErr)

	backupService.On("Run",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(backupHandler, nil)
	metadataWriter.On("deleteFolder", mock.Anything, mock.Anything).Return(nil)
	configWriter.On("Write",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return()

	handler.runFullBackup(context.Background(), time.Now())

	clientManager.AssertExpectations(t)
	backupService.AssertExpectations(t)
	backupHandler.AssertExpectations(t)
}

func TestRunIncrementalBackup_NoFullBackupYet(t *testing.T) {
	backupService := new(mockBackupService)
	clientManager := clientManagerMock()
	metadataWriter := new(mockMetadataWriter)
	configWriter := new(mockClusterConfigWriter)
	retentionManager := new(mockRetentionManager)

	handler := setupTestHandler(backupService, clientManager, metadataWriter, configWriter, retentionManager)

	handler.runIncrementalBackup(context.Background(), time.Now())

	clientManager.AssertNotCalled(t, "GetClient")
	backupService.AssertNotCalled(t, "Run")
}

func TestRunIncrementalBackup_SkipIfFullBackupInProgress(t *testing.T) {
	backupService := new(mockBackupService)
	clientManager := clientManagerMock()
	metadataWriter := new(mockMetadataWriter)
	configWriter := new(mockClusterConfigWriter)
	retentionManager := new(mockRetentionManager)

	handler := setupTestHandler(backupService, clientManager, metadataWriter, configWriter, retentionManager)

	handler.runIncrementalBackup(context.Background(), time.Now())

	clientManager.AssertNotCalled(t, "GetClient")
	backupService.AssertNotCalled(t, "Run")
}

func TestRunIncrementalBackup_SkipIfIncrementalBackupInProgress(t *testing.T) {
	backupService := new(mockBackupService)
	clientManager := clientManagerMock()
	metadataWriter := new(mockMetadataWriter)
	configWriter := new(mockClusterConfigWriter)
	retentionManager := new(mockRetentionManager)

	handler := setupTestHandler(backupService, clientManager, metadataWriter, configWriter, retentionManager)

	handler.runIncrementalBackup(context.Background(), time.Now())

	clientManager.AssertNotCalled(t, "GetClient")
	backupService.AssertNotCalled(t, "Run")
}

func TestRunIncrementalBackup_ClientError(t *testing.T) {
	backupService := new(mockBackupService)
	clientManager := new(mockClientManager)
	metadataWriter := new(mockMetadataWriter)
	configWriter := new(mockClusterConfigWriter)
	retentionManager := new(mockRetentionManager)

	handler := setupTestHandler(backupService, clientManager, metadataWriter, configWriter, retentionManager)
	now := time.Now()
	handler.registry.unregister("routine", jobTypeFull, now.Add(-1*time.Second))

	expectedErr := errors.New("client error")
	clientManager.On("GetClient", mock.Anything).Return(nil, expectedErr)

	handler.runIncrementalBackup(context.Background(), now)

	clientManager.AssertExpectations(t)
	backupService.AssertNotCalled(t, "Run")
}

func TestRunIncrementalBackup_Success(t *testing.T) {
	backupService := new(mockBackupService)
	clientManager := clientManagerMock()
	metadataWriter := new(mockMetadataWriter)
	configWriter := new(mockClusterConfigWriter)
	retentionManager := new(mockRetentionManager)

	handler := setupTestHandler(backupService, clientManager, metadataWriter, configWriter, retentionManager)
	now := time.Now()
	handler.registry.unregister("routine", jobTypeFull, now.Add(-1*time.Second))

	backupHandler := new(mockBackupHandler)
	stats := models.NewBackupStats()
	backupHandler.On("Wait", mock.Anything).Return(nil)
	backupHandler.On("GetStats").Return(stats)

	// Expect backup run for each namespace
	backupService.On("Run",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		"ns1",
		mock.Anything,
	).Return(backupHandler, nil)

	backupService.On("Run",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		"ns2",
		mock.Anything,
	).Return(backupHandler, nil)

	metadataWriter.On("writeBackupMetadata",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)
	metadataWriter.On("deleteFolder", mock.Anything, mock.Anything).Return(nil)

	handler.runIncrementalBackup(context.Background(), now)

	clientManager.AssertExpectations(t)
	backupService.AssertExpectations(t)
	backupHandler.AssertExpectations(t)
}

func TestRunFullBackup_PartialFailure(t *testing.T) {
	backupService := new(mockBackupService)
	metadataWriter := new(mockMetadataWriter)
	configWriter := new(mockClusterConfigWriter)
	clientManager := clientManagerMock()
	retentionManager := new(mockRetentionManager)

	handler := setupTestHandler(backupService, clientManager, metadataWriter, configWriter, retentionManager)

	successHandler := new(mockBackupHandler)
	successHandler.On("Wait", mock.Anything).Return(nil)
	successHandler.On("GetStats").Return(models.NewBackupStats())

	failHandler := new(mockBackupHandler)
	failHandler.On("Wait", mock.Anything).Return(errors.New("failed backup for namespace2"))

	// Set up BackupRun calls for namespaces
	backupService.On("Run",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		"ns1",
		mock.Anything,
	).Return(successHandler, nil).Once()

	backupService.On("Run",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		"ns2",
		mock.Anything,
	).Return(failHandler, nil).Once()

	metadataWriter.On("writeBackupMetadata",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil).Times(1) // Only for ns1
	metadataWriter.On("deleteFolder", mock.Anything, mock.Anything).Return(nil)
	retentionManager.On("deleteOldBackups", mock.Anything).Return(nil)
	configWriter.On("Write",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return()

	// Run full backup and expect an error for one of the namespaces
	handler.runFullBackup(context.Background(), time.Now())

	// Assertions
	successHandler.AssertExpectations(t)
	failHandler.AssertExpectations(t)
	backupService.AssertExpectations(t)
}

func clientManagerMock() *mockClientManager {
	client := &backup.Client{}
	clientManager := new(mockClientManager)
	clientManager.On("GetClient", mock.Anything).Return(client, nil)
	clientManager.On("Close", client).Return()
	return clientManager
}
