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
	"github.com/aerospike/backup-go/models"
	"github.com/prometheus/client_golang/prometheus"
	p "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func setupBaseConfig() *model.Config {
	config := model.NewConfig()
	_ = config.AddRoutine(routineName, &model.BackupRoutine{
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

	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockBackupHandler := backupexecutor.NewMockBackupHandler(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockRetentionManager := NewMockRetentionManager(ctrl)
	mockClusterConfigWriter := NewMockClusterConfigWriter(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)

	stats := models.NewBackupStats()
	stats.Start()
	stats.TotalRecords = 10
	stats.IncFiles()
	stats.ReadRecords.Add(10)

	initialState := &model.RoutineState{}
	now := time.Now()

	mockRegistry.EXPECT().GetRoutineState(routineName).Return(initialState)
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

	mockRegistry.EXPECT().register(routineName, jobTypeFull, gomock.Any())
	mockRegistry.EXPECT().unregister(routineName, jobTypeFull, gomock.Any())
	mockRetentionManager.EXPECT().deleteOldBackups(gomock.Any(), gomock.Any()).Return(nil)
	mockClusterConfigWriter.EXPECT().Write(gomock.Any(), routineName, newTimeMatcher(now)).Return(nil)

	mockBackupBackend.EXPECT().WriteBackupMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).Times(2)

	o := newOrchestrator(routineName, config, NewBackupComponents(
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
	mockRegistry.EXPECT().GetRoutineState(routineName).Return(&model.RoutineState{
		Full: &model.RunningJob{
			StartTime: time.Now(),
		},
	})

	o := newOrchestrator(routineName, config, NewBackupComponents(
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
	mockRegistry.EXPECT().GetRoutineState(routineName).Return(&model.RoutineState{})

	o := newOrchestrator(routineName, config, NewBackupComponents(
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

func TestRunIncrementalBackup_SkipWhenNoFullBackup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := setupBaseConfig()

	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockRetentionManager := NewMockRetentionManager(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)
	mockClusterConfigWriter := NewMockClusterConfigWriter(ctrl)

	mockRegistry.EXPECT().GetRoutineState(routineName).Return(&model.RoutineState{
		LastRunTime: model.NewLastBackupRun(nil, nil),
	}).AnyTimes()

	o := newOrchestrator(routineName, config, NewBackupComponents(
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
	mockRegistry.EXPECT().GetRoutineState(routineName).Return(&runningBackup).AnyTimes()

	o := newOrchestrator(routineName, config, NewBackupComponents(
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

func TestRunIncrementalBackup_Success(t *testing.T) {
	routineState := &model.RoutineState{
		// no full or incremental backups are running now
		LastRunTime: model.NewLastBackupRun(util.Ptr(time.Now()), nil),
	}

	runIncrementalBackup(t, routineState, setupBaseConfig())
}

func TestRunIncrementalBackup_AllowConcurrentFull(t *testing.T) {
	routineState := &model.RoutineState{
		Full: &model.RunningJob{
			StartTime: time.Now(),
		},
		LastRunTime: model.NewLastBackupRun(util.Ptr(time.Now()), nil),
	}

	runIncrementalBackup(t, routineState, configAllowConcurrentIncremental())
}

func TestRunIncrementalBackup_AllowConcurrentIncremental(t *testing.T) {
	routineState := &model.RoutineState{
		Incremental: &model.RunningJob{
			StartTime: time.Now(),
		},
		LastRunTime: model.NewLastBackupRun(util.Ptr(time.Now()), nil),
	}

	runIncrementalBackup(t, routineState, configAllowConcurrentIncremental())
}

func configAllowConcurrentIncremental() *model.Config {
	config := setupBaseConfig()
	routine, _ := config.Routine(routineName)
	routine.BackupPolicy.AllowConcurrentIncremental = util.Ptr(true)
	_ = config.UpdateRoutine(routineName, routine)

	return config
}

func runIncrementalBackup(t *testing.T, state *model.RoutineState, config *model.Config) {
	t.Helper()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClientManager := aerospike.NewMockClientManager(ctrl)

	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockBackupHandler := backupexecutor.NewMockBackupHandler(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockRetentionManager := NewMockRetentionManager(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)
	mockClusterConfigWriter := NewMockClusterConfigWriter(ctrl)

	stats := models.NewBackupStats()
	stats.Start()
	stats.TotalRecords = 5
	stats.IncFiles()
	stats.ReadRecords.Add(5)

	// Simulate an ongoing full backup
	mockRegistry.EXPECT().GetRoutineState(routineName).Return(state).
		Times(2) // in skipIncrementalBackup and createTimeBounds

	mockClientManager.EXPECT().GetClient(gomock.Any()).Return(mockClient, nil)
	mockClientManager.EXPECT().Close(mockClient)

	mockBackupExecutor.EXPECT().Run(
		gomock.Any(),
		mockClient,
		gomock.Any(),
		newTimeBoundsFromTimeMatcher(*state.LastRunTime.FullBackupTime()),
		gomock.Any(),
		gomock.Any(),
	).Return(mockBackupHandler, nil).Times(2)

	mockBackupHandler.EXPECT().GetStats().Return(stats).AnyTimes()
	mockBackupHandler.EXPECT().Wait(gomock.Any()).Return(nil).Times(2) // for ns1 and ns2

	mockRegistry.EXPECT().register(routineName, jobTypeIncremental, gomock.Any())
	mockRegistry.EXPECT().unregister(routineName, jobTypeIncremental, gomock.Any())

	mockBackupBackend.EXPECT().WriteBackupMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).Times(2)

	o := newOrchestrator(routineName, config, NewBackupComponents(
		mockClientManager,
		mockBackupExecutor,
		mockRegistry,
		mockRetentionManager,
		mockBackupBackend,
		mockClusterConfigWriter,
	))

	ctx := context.Background()
	now := time.Now()

	o.runIncrementalBackup(ctx, now)
	time.Sleep(10 * time.Millisecond) // time to unregister routine.

	assert.Equal(t, uint64(5), stats.TotalRecords, "Backup stats should be correct")
}
