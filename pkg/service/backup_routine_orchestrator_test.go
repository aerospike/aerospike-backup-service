package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	"github.com/prometheus/client_golang/prometheus/testutil"
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
				BaseTimeout: 100 * time.Millisecond,
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
	stats.TotalRecords.Store(10)
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

	counter := backupCounters.WithLabelValues(routineName, string(jobTypeFull), string(BackupOutcomeSuccess))
	counterValueBefore := testutil.ToFloat64(counter)
	o.runFullBackup(context.Background(), now)

	time.Sleep(10 * time.Millisecond) // time to unregister routine.
	assert.Equal(t, uint64(10), stats.TotalRecords.Load(), "Backup stats should be correct")

	counterValueAfter := testutil.ToFloat64(counter)
	assert.Equal(t, counterValueBefore+1, counterValueAfter)
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

	mockClientManager.EXPECT().GetClient(gomock.Any()).Return(mockClient, nil)
	mockClientManager.EXPECT().Close(mockClient)

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

	counter := backupCounters.WithLabelValues(routineName, string(jobTypeFull), string(BackupOutcomeSkip))
	counterValueBefore := testutil.ToFloat64(counter)

	err := o.runFullBackupInternal(ctx, now)
	assert.NoError(t, err)

	counterValueAfter := testutil.ToFloat64(counter)
	assert.Equal(t, counterValueBefore+1, counterValueAfter)
}

func TestRunFullBackupInternal_ClientConnectionFailure(t *testing.T) {
	// setup test logger
	defer func(old *slog.Logger) {
		slog.SetDefault(old)
	}(slog.Default())

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

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

	o := newOrchestrator(routineName, config, NewBackupComponents(
		mockClientManager,
		mockBackupExecutor,
		mockRegistry,
		mockRetentionManager,
		mockBackupBackend,
		mockClusterConfigWriter,
	))

	counter := backupCounters.WithLabelValues(routineName, string(jobTypeFull), string(BackupOutcomeFailure))
	counterValueBefore := testutil.ToFloat64(counter)
	o.runFullBackup(context.Background(), time.Now())

	counterValueAfter := testutil.ToFloat64(counter)
	assert.Equal(t, counterValueBefore+1, counterValueAfter)
	assert.Contains(t, buf.String(), connectionError.Error())
}

// TestRunFullBackupInternal_RaceCondition tests the race condition scenario where:
// 1. First call to runFullBackupInternal fails to get client on first try (network error).
// 2. Second call starts and successfully gets client and starts backup.
// 3. First call retries, gets client and tries to start backup but should skip it.
func TestBackupRoutineOrchestrator_runFullBackup_RaceCondition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockRetentionManager := NewMockRetentionManager(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)
	mockClusterConfigWriter := NewMockClusterConfigWriter(ctrl)
	mockClient = &backup.Client{}
	mockBackupHandler := backupexecutor.NewMockBackupHandler(ctrl)

	config := setupBaseConfig()
	registry := NewRunningBackupsRegistry(ctx, mockBackupBackend, config)
	orchestrator := newOrchestrator(routineName, config, NewBackupComponents(
		mockClientManager,
		mockBackupExecutor,
		registry,
		mockRetentionManager,
		mockBackupBackend,
		mockClusterConfigWriter,
	))

	var wg sync.WaitGroup
	wg.Add(2)
	var notFirstCall atomic.Bool

	// Mock GetClient to simulate the race condition
	// The first call (from Process A) will fail.
	// The second call (from Process B) will succeed.
	// The third call (from Process A's retry) will succeed after B has started.
	mockClientManager.EXPECT().GetClient(gomock.Any()).
		DoAndReturn(func(_ *model.AerospikeCluster) (*backup.Client, error) {
			if !notFirstCall.Swap(true) { // Process A fails
				return nil, errors.New("network error")
			}
			return mockClient, nil
		}).Times(3)

	// Expect the backup to be run twice due to the race
	mockBackupExecutor.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(mockBackupHandler, nil).Times(2)

	firstBackupTime := time.Unix(1, 0)
	secondBackupTime := time.Unix(999, 0)

	mockClientManager.EXPECT().Close(mockClient).Times(2) // both calls create a client, but only one of them runs backups
	mockClusterConfigWriter.EXPECT().Write(gomock.Any(), routineName, newTimeMatcher(secondBackupTime)).Return(nil)
	mockBackupHandler.EXPECT().GetStats().Return(models.NewBackupStats()).AnyTimes()
	mockBackupHandler.EXPECT().GetMetrics().Return(models.NewMetrics(0, 0, 0, 0)).AnyTimes()
	mockBackupHandler.EXPECT().Wait(gomock.Any()).DoAndReturn(func(_ context.Context) error {
		time.Sleep(1 * time.Second)
		return nil
	}).Times(2) // for ns1 and ns2
	mockBackupBackend.EXPECT().WriteBackupMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).Times(2)
	mockRetentionManager.EXPECT().deleteOldBackups(gomock.Any(), gomock.Any()).Return(nil)

	// Run two backup routines concurrently
	go func() {
		defer wg.Done()
		err := orchestrator.runFullBackupInternal(context.Background(), firstBackupTime)
		assert.NoError(t, err)
	}()

	go func() {
		defer wg.Done()
		// give the first goroutine a slight head start
		time.Sleep(10 * time.Millisecond)
		err := orchestrator.runFullBackupInternal(context.Background(), secondBackupTime)
		assert.NoError(t, err)
	}()

	wg.Wait()
	time.Sleep(10 * time.Millisecond) // time to unregister routine.
}

func TestSkipIncrementalBackup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	lastFullBackupTime := now.Add(-25 * time.Hour)

	tests := []struct {
		name           string
		routineState   *model.RoutineState
		concurrent     bool
		intervalCron   string
		now            time.Time
		expectedToSkip bool
	}{
		{
			name: "don't skip usually",
			routineState: &model.RoutineState{
				LastRunTime: model.NewFullBackupTime(lastFullBackupTime),
			},
			intervalCron:   "@daily",
			now:            now.Add(1 * time.Hour),
			expectedToSkip: false,
		},
		{
			name:           "skip when no full backup",
			routineState:   &model.RoutineState{LastRunTime: model.NewNoBackupTime()},
			expectedToSkip: true,
		},
		{
			name: "skip when full backup in progress",
			routineState: &model.RoutineState{
				LastRunTime: model.NewFullBackupTime(now.Add(-24 * time.Hour)),
				Full:        &model.RunningJob{StartTime: now},
			},
			expectedToSkip: true,
		},
		{
			name: "don't skip when full backup in progress and concurrent allowed",
			routineState: &model.RoutineState{
				LastRunTime: model.NewFullBackupTime(now.Add(-24 * time.Hour)),
				Full:        &model.RunningJob{StartTime: now},
			},
			concurrent:     true,
			expectedToSkip: false,
		},
		{
			name: "skip when full backup is scheduled at same time",
			routineState: &model.RoutineState{
				LastRunTime: model.NewFullBackupTime(lastFullBackupTime),
			},
			intervalCron:   "@daily",
			now:            now,
			expectedToSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := setupBaseConfig()
			routine, _ := config.Routine(routineName)
			routine.IntervalCron = tt.intervalCron
			if tt.concurrent {
				routine.BackupPolicy.ConcurrentIncremental = util.Ptr(true)
			}

			mockRegistry := NewMockRunningBackupsRegistry(ctrl)
			mockRegistry.EXPECT().GetRoutineState(routineName).Return(tt.routineState).AnyTimes()

			orchestrator := newOrchestrator(routineName, config, &BackupComponents{
				registry: mockRegistry,
			})

			assert.Equal(t, tt.expectedToSkip, orchestrator.skipIncrementalBackup(tt.now))
		})
	}
}

func TestRunIncrementalBackup_Success(t *testing.T) {
	routineState := &model.RoutineState{
		// no full or incremental backups are running now
		LastRunTime: model.NewFullBackupTime(time.Now()),
	}

	counter := backupCounters.WithLabelValues(routineName, string(jobTypeIncremental), string(BackupOutcomeSuccess))
	counterValueBefore := testutil.ToFloat64(counter)

	runIncrementalBackup(t, routineState, setupBaseConfig())

	counterValueAfter := testutil.ToFloat64(counter)
	assert.Equal(t, counterValueBefore+1, counterValueAfter)
}

func TestRunIncrementalBackup_AllowConcurrentFull(t *testing.T) {
	routineState := &model.RoutineState{
		Full: &model.RunningJob{
			StartTime: time.Now(),
		},
		LastRunTime: model.NewFullBackupTime(time.Now()),
	}

	runIncrementalBackup(t, routineState, configWithConcurrentIncremental())
}

func TestRunIncrementalBackup_ConcurrentIncremental(t *testing.T) {
	routineState := &model.RoutineState{
		Incremental: &model.RunningJob{
			StartTime: time.Now(),
		},
		LastRunTime: model.NewFullBackupTime(time.Now()),
	}

	runIncrementalBackup(t, routineState, configWithConcurrentIncremental())
}

func configWithConcurrentIncremental() *model.Config {
	config := setupBaseConfig()
	routine, _ := config.Routine(routineName)
	routine.BackupPolicy.ConcurrentIncremental = util.Ptr(true)
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
	stats.TotalRecords.Store(5)
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
	counter := backupCounters.WithLabelValues(routineName, string(jobTypeIncremental), string(BackupOutcomeSuccess))
	counterValueBefore := testutil.ToFloat64(counter)

	o.runIncrementalBackup(ctx, now)
	time.Sleep(10 * time.Millisecond) // time to unregister routine.

	assert.Equal(t, uint64(5), stats.TotalRecords.Load(), "Backup stats should be correct")
	counterValueAfter := testutil.ToFloat64(counter)
	assert.Equal(t, counterValueBefore+1, counterValueAfter)
}
