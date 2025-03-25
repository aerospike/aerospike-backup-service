package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupBaseConfig() *model.Config {
	config := model.NewConfig()
	_ = config.AddRoutine("routine1", &model.BackupRoutine{
		Storage:       &model.LocalStorage{Path: "test-path"},
		SourceCluster: &model.AerospikeCluster{},
		BackupPolicy: &model.BackupPolicy{
			RetryPolicy: &models.RetryPolicy{
				BaseTimeout: 1 * time.Millisecond,
				MaxRetries:  1,
				Multiplier:  1,
			},
		},
		IntervalCron: "@daily",
		Namespaces:   []string{"ns1", "ns2"},
	})
	return config
}

func TestRunFullBackupInternal_Success(t *testing.T) {
	config := setupBaseConfig()

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
	initialState := &model.RoutineState{}
	mockRegistry.On("GetRoutineState", mock.Anything).Return(initialState)

	var registeredHandler CancelableBackupHandler
	mockRegistry.On("register", "routine1", jobTypeFull, mock.Anything).Run(func(args mock.Arguments) {
		registeredHandler = args.Get(2).(CancelableBackupHandler)
	}).Return()
	mockRegistry.On("unregister", "routine1", jobTypeFull, mock.Anything).Return()

	mockRetentionManager := new(mockRetentionManager)
	mockRetentionManager.On("deleteOldBackups", mock.Anything, mock.Anything).Return(nil)

	mockClusterConfigWriter := new(mockClusterConfigWriter)
	var writtenTimestamp time.Time
	mockClusterConfigWriter.On("Write", mock.Anything, "routine1", mock.Anything).Run(func(args mock.Arguments) {
		writtenTimestamp = args.Get(2).(time.Time)
	}).Return()

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

	err := o.runFullBackupInternal(ctx, now)
	assert.NoError(t, err)

	mockClientManager.AssertExpectations(t)
	mockBackupExecutor.AssertExpectations(t)
	mockBackupHandler.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
	mockRetentionManager.AssertExpectations(t)
	mockClusterConfigWriter.AssertExpectations(t)

	assert.NotNil(t, registeredHandler, "Backup handler should be registered")
	assert.Equal(t, now.Unix(), writtenTimestamp.Unix(), "Timestamp should match execution time")
	assert.Equal(t, uint64(10), mockBackupHandler.GetStats().TotalRecords, "Backup stats should be correct")
}

func TestRunFullBackupInternal_SkipWhenBackupInProgress(t *testing.T) {
	config := setupBaseConfig()

	mockRegistry := new(MockRunningBackupsRegistry)
	// Simulate an ongoing full backup
	mockRegistry.On("GetRoutineState", mock.Anything).Return(&model.RoutineState{
		Full: &model.RunningJob{
			StartTime: time.Now(),
		},
	})

	o := newOrchestrator("routine1", NewBackupComponents(
		new(mockClientManager),
		new(mockBackupExecutor),
		mockRegistry,
		new(mockRetentionManager),
		new(MockBackupBackendService),
		new(mockClusterConfigWriter),
		config,
	))

	ctx := context.Background()
	now := time.Now()

	err := o.runFullBackupInternal(ctx, now)
	assert.NoError(t, err)

	mockRegistry.AssertExpectations(t)
}

func TestRunFullBackupInternal_ClientConnectionFailure(t *testing.T) {

	config := setupBaseConfig()

	mockClientManager := new(mockClientManager)
	mockClientManager.On("GetClient", mock.Anything).Return(nil, errors.New("connection failed"))

	mockRegistry := new(MockRunningBackupsRegistry)
	mockRegistry.On("GetRoutineState", mock.Anything).Return(&model.RoutineState{})

	o := newOrchestrator("routine1", NewBackupComponents(
		mockClientManager,
		new(mockBackupExecutor),
		mockRegistry,
		new(mockRetentionManager),
		new(MockBackupBackendService),
		new(mockClusterConfigWriter),
		config,
	))

	ctx := context.Background()
	now := time.Now()

	err := o.runFullBackupInternal(ctx, now)

	assert.Error(t, err, "Should return error on client connection failure")
	assert.Contains(t, err.Error(), "cannot get backup client")
	mockClientManager.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
}
