package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go/models"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func testRoutine() *model.BackupRoutine {
	return &model.BackupRoutine{
		Name:          routineName,
		Storage:       &model.LocalStorage{Path: "test-path"},
		SourceCluster: &model.AerospikeCluster{},
		BackupPolicy: &model.BackupPolicy{
			RetryPolicy: &model.RetryPolicy{
				BaseTimeout: ptr.Of(100 * time.Millisecond),
				MaxRetries:  ptr.Of(1),
				Multiplier:  ptr.Of(1.0),
			},
			WithClusterConfig: ptr.Of(true),
		},
		IntervalCron: "@daily",
		Namespaces:   []string{"ns1", "ns2"},
	}
}

func TestRunFullBackupInternal_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := testRoutine()
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
	mockRegistry.EXPECT().GetRoutineState(routine).Return(initialState)
	mockClientManager.EXPECT().GetClient(gomock.Any(), gomock.Any()).Return(mockClient, nil)
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

	registryWG := sync.WaitGroup{}
	registryWG.Add(2)
	mockRegistry.EXPECT().register(routineName, jobTypeFull, gomock.Any()).Do(func(_, _, _ any) {
		registryWG.Done()
	})
	mockRegistry.EXPECT().unregister(routineName, jobTypeFull, gomock.Any()).Do(func(_, _, _ any) {
		registryWG.Done()
	})
	mockRetentionManager.EXPECT().deleteOldBackups(gomock.Any(), gomock.Any()).Return(nil)

	now := time.Now()
	mockClusterConfigWriter.EXPECT().Write(gomock.Any(), routine, newTimeMatcher(now)).Return(nil)

	mockBackupBackend.EXPECT().WriteBackupMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).Times(2) // for ns1 and ns2

	o := newOrchestrator(routine, NewBackupComponents(
		mockClientManager,
		mockBackupExecutor,
		mockRegistry,
		mockRetentionManager,
		mockBackupBackend,
		mockClusterConfigWriter,
	), NewPathService(nil))

	backupCounters.Reset()
	o.runFullBackup(context.Background(), now)

	registryWG.Wait()
	assert.Equal(t, uint64(10), stats.TotalRecords.Load(), "Backup stats should be correct")

	assert.Equal(t, 1, prometheusCounter(jobTypeFull, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(jobTypeFull, BackupOutcomeSkip))
	assert.Zero(t, prometheusCounter(jobTypeFull, BackupOutcomeFailure))
}

func TestRunFullBackupInternal_SkipWhenBackupInProgress(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := testRoutine()

	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockRetentionManager := NewMockRetentionManager(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)
	mockClusterConfigWriter := NewMockClusterConfigWriter(ctrl)

	// Simulate an ongoing full backup
	mockRegistry.EXPECT().GetRoutineState(routine).Return(&model.RoutineState{
		Full: &model.RunningJob{
			StartTime: time.Now(),
		},
	})

	o := newOrchestrator(routine, NewBackupComponents(
		mockClientManager,
		mockBackupExecutor,
		mockRegistry,
		mockRetentionManager,
		mockBackupBackend,
		mockClusterConfigWriter,
	), NewPathService(nil))

	ctx := context.Background()
	now := time.Now()

	backupCounters.Reset()

	o.runFullBackup(ctx, now)

	assert.Zero(t, prometheusCounter(jobTypeFull, BackupOutcomeSuccess))
	assert.Equal(t, 1, prometheusCounter(jobTypeFull, BackupOutcomeSkip))
	assert.Zero(t, prometheusCounter(jobTypeFull, BackupOutcomeFailure))
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

	routine := testRoutine()

	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockRetentionManager := NewMockRetentionManager(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)
	mockClusterConfigWriter := NewMockClusterConfigWriter(ctrl)

	connectionError := errors.New("connection failed")
	mockClientManager.EXPECT().GetClient(gomock.Any(), gomock.Any()).Return(nil, connectionError).Times(1)

	initialState := &model.RoutineState{}
	mockRegistry.EXPECT().GetRoutineState(routine).Return(initialState)

	o := newOrchestrator(routine, NewBackupComponents(
		mockClientManager,
		mockBackupExecutor,
		mockRegistry,
		mockRetentionManager,
		mockBackupBackend,
		mockClusterConfigWriter,
	), NewPathService(nil))

	backupCounters.Reset()
	o.runFullBackup(context.Background(), time.Now())

	assert.Contains(t, buf.String(), connectionError.Error())

	assert.Zero(t, prometheusCounter(jobTypeFull, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(jobTypeFull, BackupOutcomeSkip))
	assert.Equal(t, 1, prometheusCounter(jobTypeFull, BackupOutcomeFailure))
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
			routine := testRoutine()
			routine.IntervalCron = tt.intervalCron
			if tt.concurrent {
				routine.BackupPolicy.ConcurrentIncremental = ptr.Of(true)
			}

			mockRegistry := NewMockRunningBackupsRegistry(ctrl)
			mockRegistry.EXPECT().GetRoutineState(routine).Return(tt.routineState).AnyTimes()

			orchestrator := newOrchestrator(routine, &BackupComponents{
				registry: mockRegistry,
			}, NewPathService(nil))

			assert.Equal(t, tt.expectedToSkip, orchestrator.skipIncrementalBackup(tt.now))
		})
	}
}

func TestRunIncrementalBackup_Success(t *testing.T) {
	routineState := &model.RoutineState{
		// no full or incremental backups are running now
		LastRunTime: model.NewFullBackupTime(time.Now()),
	}

	backupCounters.Reset()

	runIncrementalBackup(t, routineState, testRoutine())

	assert.Equal(t, 1, prometheusCounter(jobTypeIncremental, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(jobTypeIncremental, BackupOutcomeFailure))
	assert.Zero(t, prometheusCounter(jobTypeIncremental, BackupOutcomeSkip))
}

func TestRunIncrementalBackup_Skip(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := testRoutine()
	routineState := &model.RoutineState{
		// no full or incremental backups exists
		LastRunTime: model.NewNoBackupTime(),
	}

	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockRegistry.EXPECT().GetRoutineState(routine).Return(routineState)

	o := newOrchestrator(routine, NewBackupComponents(
		nil,
		nil,
		mockRegistry,
		nil,
		nil,
		nil,
	), NewPathService(nil))

	backupCounters.Reset()

	o.runIncrementalBackup(context.Background(), time.Now())

	assert.Equal(t, 1, prometheusCounter(jobTypeIncremental, BackupOutcomeSkip))
	assert.Zero(t, prometheusCounter(jobTypeIncremental, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(jobTypeIncremental, BackupOutcomeFailure))
}

func TestRunIncrementalBackup_AllowConcurrentFull(t *testing.T) {
	routineState := &model.RoutineState{
		Full: &model.RunningJob{
			StartTime: time.Now(),
		},
		LastRunTime: model.NewFullBackupTime(time.Now()),
	}

	backupCounters.Reset()

	routine := testRoutine()
	routine.BackupPolicy.ConcurrentIncremental = ptr.Of(true)
	runIncrementalBackup(t, routineState, routine)

	assert.Equal(t, 1, prometheusCounter(jobTypeIncremental, BackupOutcomeSuccess))
}

func TestRunIncrementalBackup_ConcurrentIncremental(t *testing.T) {
	routineState := &model.RoutineState{
		Incremental: &model.RunningJob{
			StartTime: time.Now(),
		},
		LastRunTime: model.NewFullBackupTime(time.Now()),
	}

	backupCounters.Reset()

	routine := testRoutine()
	routine.BackupPolicy.ConcurrentIncremental = ptr.Of(true)
	runIncrementalBackup(t, routineState, routine)

	assert.Equal(t, 1, prometheusCounter(jobTypeIncremental, BackupOutcomeSuccess))
}

func runIncrementalBackup(t *testing.T, state *model.RoutineState, routine *model.BackupRoutine) {
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
	mockRegistry.EXPECT().GetRoutineState(routine).Return(state).
		Times(2) // in skipIncrementalBackup and createTimeBounds

	mockClientManager.EXPECT().GetClient(gomock.Any(), gomock.Any()).Return(mockClient, nil)
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

	o := newOrchestrator(routine, NewBackupComponents(
		mockClientManager,
		mockBackupExecutor,
		mockRegistry,
		mockRetentionManager,
		mockBackupBackend,
		mockClusterConfigWriter,
	), NewPathService(nil))

	o.runIncrementalBackup(context.Background(), time.Now())
	time.Sleep(10 * time.Millisecond) // time to unregister routine.

	assert.Equal(t, uint64(5), stats.TotalRecords.Load(), "Backup stats should be correct")
}

func prometheusCounter(jobType jobType, outcome BackupOutcome) int {
	counter := backupCounters.WithLabelValues(routineName, string(jobType), string(outcome))
	return int(testutil.ToFloat64(counter))
}
