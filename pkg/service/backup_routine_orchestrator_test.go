package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	"github.com/prometheus/client_golang/prometheus"
	p "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := setupBaseConfig()

	// Setup mocks
	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockClient := &backup.Client{}
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockBackupHandler := backupexecutor.NewMockBackupHandler(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockRetentionManager := NewMockRetentionManager(ctrl)
	mockClusterConfigWriter := NewMockClusterConfigWriter(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)

	// Setup stats
	stats := models.NewBackupStats()
	stats.Start()
	stats.TotalRecords = 10
	stats.IncFiles()
	stats.ReadRecords.Add(10)

	initialState := &model.RoutineState{}
	now := time.Now()

	// Setup expectations
	mockRegistry.EXPECT().GetRoutineState("routine1").Return(initialState)
	mockClientManager.EXPECT().GetClient(gomock.Any()).Return(mockClient, nil)
	mockClientManager.EXPECT().Close(mockClient)

	mockBackupExecutor.EXPECT().Run(
		gomock.Any(),
		mockClient,
		gomock.Any(),
		gomock.Any(),
		"ns1",
		gomock.Any(),
	).Return(mockBackupHandler, nil)

	mockBackupExecutor.EXPECT().Run(
		gomock.Any(),
		mockClient,
		gomock.Any(),
		gomock.Any(),
		"ns2",
		gomock.Any(),
	).Return(mockBackupHandler, nil)

	mockBackupHandler.EXPECT().GetStats().Return(stats).AnyTimes()
	mockBackupHandler.EXPECT().Wait(gomock.Any()).Return(nil).Times(2) // for ns1 and ns2

	mockRegistry.EXPECT().register("routine1", jobTypeFull, gomock.Any())
	mockRegistry.EXPECT().unregister("routine1", jobTypeFull, gomock.Any())
	mockRetentionManager.EXPECT().deleteOldBackups(gomock.Any(), gomock.Any()).Return(nil)
	mockClusterConfigWriter.EXPECT().Write(gomock.Any(), "routine1", newTimeMatcher(now)).Return(nil)

	mockBackupBackend.EXPECT().WriteBackupMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).Times(2)

	o := newOrchestrator("routine1", config, NewBackupComponents(
		mockClientManager,
		mockBackupExecutor,
		mockRegistry,
		mockRetentionManager,
		mockBackupBackend,
		mockClusterConfigWriter,
	))

	ctx := context.Background()
	err := o.runFullBackupInternal(ctx, now)
	time.Sleep(10 * time.Millisecond) // time to unregister routine.

	assert.NoError(t, err)
	assert.Equal(t, uint64(10), stats.TotalRecords, "Backup stats should be correct")
}

func TestRunFullBackupInternal_SkipWhenBackupInProgress(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := setupBaseConfig()

	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockRetentionManager := NewMockRetentionManager(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)
	mockClusterConfigWriter := NewMockClusterConfigWriter(ctrl)

	// Simulate an ongoing full backup
	mockRegistry.EXPECT().GetRoutineState("routine1").Return(&model.RoutineState{
		Full: &model.RunningJob{
			StartTime: time.Now(),
		},
	})

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
}

func TestRunFullBackupInternal_ClientConnectionFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := setupBaseConfig()

	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockRetentionManager := NewMockRetentionManager(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)
	mockClusterConfigWriter := NewMockClusterConfigWriter(ctrl)

	connectionError := errors.New("connection failed")
	mockClientManager.EXPECT().GetClient(gomock.Any()).Return(nil, connectionError).Times(2) // retries
	mockRegistry.EXPECT().GetRoutineState("routine1").Return(&model.RoutineState{})

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

	assert.Error(t, err, "Should return error on client connection failure")
	assert.Contains(t, err.Error(), "cannot get backup client")
}

func TestRunIncrementalBackupInternal_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := setupBaseConfig()

	// Setup last full backup time
	lastFullBackupTime := time.Now().Add(-24 * time.Hour)
	initialState := &model.RoutineState{
		LastRunTime: model.NewLastBackupRun(&lastFullBackupTime, nil),
	}

	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockClient := &backup.Client{}
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockBackupHandler := backupexecutor.NewMockBackupHandler(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockRetentionManager := NewMockRetentionManager(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)
	mockClusterConfigWriter := NewMockClusterConfigWriter(ctrl)

	// Setup stats
	stats := models.NewBackupStats()
	stats.Start()
	stats.TotalRecords = 5
	stats.IncFiles()
	stats.ReadRecords.Add(5)

	mockRegistry.EXPECT().GetRoutineState("routine1").Return(initialState)
	mockClientManager.EXPECT().GetClient(gomock.Any()).Return(mockClient, nil)
	mockClientManager.EXPECT().Close(mockClient)

	mockBackupExecutor.EXPECT().Run(
		gomock.Any(),
		mockClient,
		gomock.Any(),
		newTimeBoundsFromTimeMatcher(lastFullBackupTime),
		"ns1",
		gomock.Any(),
	).Return(mockBackupHandler, nil)

	mockBackupExecutor.EXPECT().Run(
		gomock.Any(),
		mockClient,
		gomock.Any(),
		newTimeBoundsFromTimeMatcher(lastFullBackupTime),
		"ns2",
		gomock.Any(),
	).Return(mockBackupHandler, nil)

	mockBackupHandler.EXPECT().GetStats().Return(stats).AnyTimes()
	mockBackupHandler.EXPECT().Wait(gomock.Any()).Return(nil).Times(2) // for ns1 and ns2

	mockRegistry.EXPECT().register("routine1", jobTypeIncremental, gomock.Any())
	mockRegistry.EXPECT().unregister("routine1", jobTypeIncremental, gomock.Any())

	mockBackupBackend.EXPECT().WriteBackupMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).Times(2)

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

	err := o.runIncrementalBackupInternal(ctx, now)
	time.Sleep(10 * time.Millisecond) // time to unregister routine.

	assert.NoError(t, err)
	assert.Equal(t, uint64(5), stats.TotalRecords, "Backup stats should be correct")
}

func TestRunIncrementalBackup_SkipWhenNoFullBackup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := setupBaseConfig()

	// Create and setup mocks
	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockRetentionManager := NewMockRetentionManager(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)
	mockClusterConfigWriter := NewMockClusterConfigWriter(ctrl)

	mockRegistry.EXPECT().GetRoutineState("routine1").Return(&model.RoutineState{
		LastRunTime: model.NewLastBackupRun(nil, nil),
	}).AnyTimes()

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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := setupBaseConfig()

	// Create and setup mocks
	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockRetentionManager := NewMockRetentionManager(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)
	mockClusterConfigWriter := NewMockClusterConfigWriter(ctrl)

	// Simulate an ongoing full backup
	runningBackup := model.RoutineState{
		LastRunTime: model.NewLastBackupRun(util.Ptr(time.Now().Add(-24*time.Hour)), nil),
		Full: &model.RunningJob{
			StartTime: time.Now(),
		},
	}
	mockRegistry.EXPECT().GetRoutineState("routine1").Return(&runningBackup).AnyTimes()

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

	skippedBefore := getCounterValue(incrBackupSkippedCounter)
	o.runIncrementalBackup(ctx, now)

	skipped := getCounterValue(incrBackupSkippedCounter)
	assert.Equal(t, skippedBefore+1, skipped)
}
