package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/aerospike/backup-go/models"
	"github.com/prometheus/client_golang/prometheus"
	p "github.com/prometheus/client_model/go"
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
			WithClusterConfig: util.Ptr(true),
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
	}).Return(nil)

	mockBackupBackend := new(MockBackupBackendService)
	mockBackupBackend.On("WriteBackupMetadata", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	o := newOrchestrator("routine1", config, NewBackupComponents(
		mockClientManager,
		mockBackupExecutor,
		mockRegistry,
		mockRetentionManager,
		mockBackupBackend,
		mockClusterConfigWriter,
	))

	ctx := context.Background()
	now := time.Now()

	err := o.runFullBackupInternal(ctx, now)
	assert.NoError(t, err)

	mockClientManager.AssertExpectations(t)
	mockBackupExecutor.AssertExpectations(t)
	mockBackupHandler.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
	assert.Eventually(t, func() bool {
		return mockRetentionManager.AssertExpectations(t)
	}, time.Second, time.Millisecond*10)

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

	o := newOrchestrator("routine1", config, NewBackupComponents(
		new(mockClientManager),
		new(mockBackupExecutor),
		mockRegistry,
		new(mockRetentionManager),
		new(MockBackupBackendService),
		new(mockClusterConfigWriter),
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

	o := newOrchestrator("routine1", config,
		NewBackupComponents(
			mockClientManager,
			new(mockBackupExecutor),
			mockRegistry,
			new(mockRetentionManager),
			new(MockBackupBackendService),
			new(mockClusterConfigWriter),
		))

	ctx := context.Background()
	now := time.Now()

	err := o.runFullBackupInternal(ctx, now)

	assert.Error(t, err, "Should return error on client connection failure")
	assert.Contains(t, err.Error(), "cannot get backup client")
	mockClientManager.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
}

func TestRunIncrementalBackupInternal_Success(t *testing.T) {
	config := setupBaseConfig()

	// Setup last full backup time
	lastFullBackupTime := time.Now().Add(-24 * time.Hour)
	initialState := &model.RoutineState{
		LastRunTime: model.NewLastBackupRun(&lastFullBackupTime, nil),
	}

	mockClientManager, mockClient := clientManagerMock()

	mockBackupHandler := new(mockBackupHandler)
	stats := models.NewBackupStats()
	stats.Start()
	stats.TotalRecords = 5
	stats.IncFiles()
	stats.ReadRecords.Add(5)
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
	).Return(mockBackupHandler, nil).Run(func(args mock.Arguments) {
		timeBounds := args.Get(3).(model.TimeBounds)
		assert.Equal(t, lastFullBackupTime, *timeBounds.FromTime, "FromTime should match last full backup time")
	})

	mockRegistry := new(MockRunningBackupsRegistry)
	mockRegistry.On("GetRoutineState", mock.Anything).Return(initialState)

	var registeredHandler CancelableBackupHandler
	mockRegistry.On("register", "routine1", jobTypeIncremental, mock.Anything).Run(func(args mock.Arguments) {
		registeredHandler = args.Get(2).(CancelableBackupHandler)
	}).Return()
	mockRegistry.On("unregister", "routine1", jobTypeIncremental, mock.Anything).Return()

	mockBackupBackend := new(MockBackupBackendService)
	mockBackupBackend.On("WriteBackupMetadata", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	o := newOrchestrator("routine1", config, NewBackupComponents(
		mockClientManager,
		mockBackupExecutor,
		mockRegistry,
		new(mockRetentionManager),
		mockBackupBackend,
		new(mockClusterConfigWriter),
	))

	ctx := context.Background()
	now := time.Now()

	err := o.runIncrementalBackupInternal(ctx, now)
	assert.NoError(t, err)

	mockClientManager.AssertExpectations(t)
	mockBackupExecutor.AssertExpectations(t)
	mockBackupHandler.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)

	assert.NotNil(t, registeredHandler, "Backup handler should be registered")
	assert.Equal(t, uint64(5), mockBackupHandler.GetStats().TotalRecords, "Backup stats should be correct")
}

func TestRunIncrementalBackup_SkipWhenNoFullBackup(t *testing.T) {
	config := setupBaseConfig()

	mockRegistry := new(MockRunningBackupsRegistry)
	mockRegistry.On("GetRoutineState", mock.Anything).Return(&model.RoutineState{
		LastRunTime: model.NewLastBackupRun(nil, nil),
	})

	o := newOrchestrator("routine1", config, NewBackupComponents(
		new(mockClientManager),
		new(mockBackupExecutor),
		mockRegistry,
		new(mockRetentionManager),
		new(MockBackupBackendService),
		new(mockClusterConfigWriter),
	))

	ctx := context.Background()
	now := time.Now()

	skippedBefore := getCounterValue(incrBackupSkippedCounter)
	o.runIncrementalBackup(ctx, now)

	skipped := getCounterValue(incrBackupSkippedCounter)
	assert.Equal(t, skippedBefore+1, skipped)
}

func getCounterValue(c prometheus.Counter) int {
	m := &p.Metric{}
	_ = c.Write(m)
	return int(m.GetCounter().GetValue())
}

func TestRunIncrementalBackup_SkipWhenFullBackupInProgress(t *testing.T) {
	config := setupBaseConfig()

	mockRegistry := new(MockRunningBackupsRegistry)
	// Simulate an ongoing full backup
	mockRegistry.On("GetRoutineState", mock.Anything).Return(&model.RoutineState{
		LastRunTime: model.NewLastBackupRun(util.Ptr(time.Now().Add(-24*time.Hour)), nil),
		Full: &model.RunningJob{
			StartTime: time.Now(),
		},
	})

	o := newOrchestrator("routine1", config, NewBackupComponents(
		new(mockClientManager),
		new(mockBackupExecutor),
		mockRegistry,
		new(mockRetentionManager),
		new(MockBackupBackendService),
		new(mockClusterConfigWriter),
	))

	ctx := context.Background()
	now := time.Now()

	o.runIncrementalBackup(ctx, now)

	mockRegistry.AssertExpectations(t)

	skippedBefore := getCounterValue(incrBackupSkippedCounter)
	o.runIncrementalBackup(ctx, now)

	skipped := getCounterValue(incrBackupSkippedCounter)
	assert.Equal(t, skippedBefore+1, skipped)
}
