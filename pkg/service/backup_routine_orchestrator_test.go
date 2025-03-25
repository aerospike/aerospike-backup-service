package service

import (
	"context"
	"github.com/aerospike/backup-go/models"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRunFullBackupInternal_Success(t *testing.T) {
	// Setup
	var config = model.NewConfig()
	_ = config.AddRoutine("routine1", &model.BackupRoutine{
		Storage:       &model.LocalStorage{Path: "test-path"},
		SourceCluster: &model.AerospikeCluster{},
		BackupPolicy:  &model.BackupPolicy{},
		IntervalCron:  "@daily",
		Namespaces:    []string{"ns1", "ns2"},
	})

	mockClientManager, mockClient := clientManagerMock()

	mockBackupHandler := new(mockBackupHandler)
	stats := models.NewBackupStats()
	stats.Start()
	stats.TotalRecords = 10
	stats.IncFiles()
	stats.ReadRecords.Add(10)
	mockBackupHandler.On("GetStats").Return(stats)
	mockBackupHandler.On("Wait", mock.Anything).Return(nil)

	mockBackupExecutor := new(mockBackupExecutor)
	mockBackupExecutor.On("Run",
		mock.Anything,
		mockClient,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(mockBackupHandler, nil)

	mockRegistry := new(MockRunningBackupsRegistry)
	mockRegistry.On("GetRoutineState", mock.Anything).Return(&model.RoutineState{})
	mockRegistry.On("register", mock.Anything, mock.Anything, mock.Anything).Return()
	mockRegistry.On("unregister", mock.Anything, mock.Anything, mock.Anything).Return()

	mockRetentionManager := new(mockRetentionManager)
	mockRetentionManager.On("deleteOldBackups", mock.Anything, mock.Anything).Return(nil)

	mockClusterConfigWriter := new(mockClusterConfigWriter)
	mockClusterConfigWriter.On("Write", mock.Anything, mock.Anything, mock.Anything).Return()

	mockBackupBackend := new(MockBackupBackendService)
	mockBackupBackend.On("WriteBackupMetadata", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	o := newOrchestrator("routine1", NewBackupComponents(
		mockClientManager,
		mockBackupExecutor,
		mockRegistry,
		mockRetentionManager,
		mockBackupBackend,
		mockClusterConfigWriter,
		config,
	))

	ctx := context.Background()
	now := time.Now()

	// Execute
	err := o.runFullBackupInternal(ctx, now)

	// Assertions
	assert.NoError(t, err)

	mockClientManager.AssertExpectations(t)
	mockBackupExecutor.AssertExpectations(t)
	mockBackupHandler.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
	mockRetentionManager.AssertExpectations(t)
	mockClusterConfigWriter.AssertExpectations(t)
}
