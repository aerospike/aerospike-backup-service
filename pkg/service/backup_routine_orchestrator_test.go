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
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

var mockClient = &backup.Client{}

func testRoutine() *model.BackupRoutine {
	return &model.BackupRoutine{
		Name:          routineName,
		Storage:       &model.LocalStorage{Path: "test-path"},
		SourceCluster: &model.AerospikeCluster{},
		BackupPolicy: &model.BackupPolicy{
			RetryPolicy: &model.RetryPolicy{
				BaseTimeout: optional.Of(100 * time.Millisecond),
				MaxRetries:  optional.Of(1),
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
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)

	stats := models.NewBackupStats()
	stats.Start()
	stats.TotalRecords.Store(10)
	stats.IncFiles()
	stats.ReadRecords.Add(10)

	mockClientManager.EXPECT().GetClient(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockClient, nil).Times(2)
	mockClientManager.EXPECT().Close(mockClient).Times(2)

	mockBackupExecutor.EXPECT().Run(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		"ns1",
		gomock.Any(),
	).Return(mockBackupHandler, nil)

	mockBackupExecutor.EXPECT().Run(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		"ns2",
		gomock.Any(),
	).Return(mockBackupHandler, nil)

	mockBackupHandler.EXPECT().GetStats().Return(stats).AnyTimes()
	mockBackupHandler.EXPECT().Wait(gomock.Any()).Return(nil).Times(2) // for ns1 and ns2

	registryWG := sync.WaitGroup{}
	registryWG.Add(2)
	mockRegistry.EXPECT().register(routineName, model.BackupJobTypeFull, gomock.Any()).Do(func(_, _, _ any) {
		registryWG.Done()
	})
	now := time.Now()
	mockCompletionHandler.EXPECT().
		OnSuccess(gomock.Any(), routine, model.BackupJobTypeFull, newTimeMatcher(now), gomock.Any()).Do(func(_, _, _, _, _ any) {
		registryWG.Done()
	})

	mockBackupBackend.EXPECT().WriteBackupMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).Times(2) // for ns1 and ns2

	startController := NewMockStartController(ctrl)
	startController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	nsExec := NewNamespaceBackupExecutor(mockClientManager, mockBackupExecutor, mockBackupBackend, NewPathService(ptr.Of(model.TimestampFormatISO)))
	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, startController, nsExec)

	backupCounters.Reset()
	p.RunBackup(t.Context(), routine, now, model.BackupJobTypeFull)

	registryWG.Wait()
	assert.Equal(t, uint64(10), stats.TotalRecords.Load(), "Backup stats should be correct")

	assert.Equal(t, 1, prometheusCounter(model.BackupJobTypeFull, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(model.BackupJobTypeFull, BackupOutcomeSkip))
	assert.Zero(t, prometheusCounter(model.BackupJobTypeFull, BackupOutcomeFailure))
}

func TestRunFullBackupInternal_SkipWhenBackupInProgress(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := testRoutine()

	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)

	startController := NewMockStartController(ctrl)
	startController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, errBackupSkipped)

	nsExec := NewNamespaceBackupExecutor(mockClientManager, mockBackupExecutor, mockBackupBackend, NewPathService(nil))
	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, startController, nsExec)

	backupCounters.Reset()

	p.RunBackup(t.Context(), routine, time.Now(), model.BackupJobTypeFull)

	assert.Zero(t, prometheusCounter(model.BackupJobTypeFull, BackupOutcomeSuccess))
	assert.Equal(t, 1, prometheusCounter(model.BackupJobTypeFull, BackupOutcomeSkip))
	assert.Zero(t, prometheusCounter(model.BackupJobTypeFull, BackupOutcomeFailure))
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
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)

	connectionError := errors.New("connection failed")
	mockClientManager.EXPECT().GetClient(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, connectionError).Times(2)

	startController := NewMockStartController(ctrl)
	startController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	nsExec := NewNamespaceBackupExecutor(mockClientManager, mockBackupExecutor, mockBackupBackend, NewPathService(nil))
	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, startController, nsExec)

	backupCounters.Reset()
	p.RunBackup(t.Context(), routine, time.Now(), model.BackupJobTypeFull)

	assert.Contains(t, buf.String(), connectionError.Error())

	assert.Zero(t, prometheusCounter(model.BackupJobTypeFull, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(model.BackupJobTypeFull, BackupOutcomeSkip))
	assert.Equal(t, 1, prometheusCounter(model.BackupJobTypeFull, BackupOutcomeFailure))
}

func TestRunFullBackupInternal_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := testRoutine()

	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)

	mockClientManager.EXPECT().GetClient(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, context.Canceled).Times(2)

	startController := NewMockStartController(ctrl)
	startController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	nsExec := NewNamespaceBackupExecutor(mockClientManager, mockBackupExecutor, mockBackupBackend, NewPathService(ptr.Of(model.TimestampFormatISO)))
	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, startController, nsExec)

	backupCounters.Reset()
	p.RunBackup(t.Context(), routine, time.Now(), model.BackupJobTypeFull)

	assert.Zero(t, prometheusCounter(model.BackupJobTypeFull, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(model.BackupJobTypeFull, BackupOutcomeSkip))
	assert.Zero(t, prometheusCounter(model.BackupJobTypeFull, BackupOutcomeFailure))
	assert.Equal(t, 1, prometheusCounter(model.BackupJobTypeFull, BackupOutcomeCanceled))
}

func TestRunIncrementalBackup_Success(t *testing.T) {
	routineState := model.RoutineState{
		// no full or incremental backups are running now
		LastRunTime: model.NewFullBackupTime(time.Now()),
	}

	backupCounters.Reset()

	runIncrementalBackup(t, routineState, testRoutine())

	assert.Equal(t, 1, prometheusCounter(model.BackupJobTypeIncremental, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(model.BackupJobTypeIncremental, BackupOutcomeFailure))
	assert.Zero(t, prometheusCounter(model.BackupJobTypeIncremental, BackupOutcomeSkip))
}

func TestRunIncrementalBackup_Skip(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := testRoutine()
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)

	startController := NewMockStartController(ctrl)
	startController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, errBackupSkipped)

	nsExec := NewNamespaceBackupExecutor(nil, nil, nil, NewPathService(nil))
	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, startController, nsExec)

	backupCounters.Reset()

	p.RunBackup(t.Context(), routine, time.Now(), model.BackupJobTypeIncremental)

	assert.Equal(t, 1, prometheusCounter(model.BackupJobTypeIncremental, BackupOutcomeSkip))
	assert.Zero(t, prometheusCounter(model.BackupJobTypeIncremental, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(model.BackupJobTypeIncremental, BackupOutcomeFailure))
}

func TestRunIncrementalBackup_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := testRoutine()

	mockClientManager := aerospike.NewMockClientManager(ctrl)
	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)

	mockClientManager.EXPECT().GetClient(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, context.DeadlineExceeded).Times(2)

	startController := NewMockStartController(ctrl)
	startController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	nsExec := NewNamespaceBackupExecutor(mockClientManager, mockBackupExecutor, mockBackupBackend, NewPathService(nil))
	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, startController, nsExec)

	backupCounters.Reset()
	p.RunBackup(t.Context(), routine, time.Now(), model.BackupJobTypeIncremental)

	assert.Zero(t, prometheusCounter(model.BackupJobTypeIncremental, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(model.BackupJobTypeIncremental, BackupOutcomeSkip))
	assert.Zero(t, prometheusCounter(model.BackupJobTypeIncremental, BackupOutcomeFailure))
	assert.Equal(t, 1, prometheusCounter(model.BackupJobTypeIncremental, BackupOutcomeCanceled))
}

func TestRunIncrementalBackup_AllowConcurrentFull(t *testing.T) {
	routineState := model.RoutineState{
		Full: &model.RunningJob{
			StartTime: time.Now(),
		},
		LastRunTime: model.NewFullBackupTime(time.Now()),
	}

	backupCounters.Reset()

	routine := testRoutine()
	routine.BackupPolicy.ConcurrentIncremental = ptr.Of(true)
	runIncrementalBackup(t, routineState, routine)

	assert.Equal(t, 1, prometheusCounter(model.BackupJobTypeIncremental, BackupOutcomeSuccess))
}

func TestRunIncrementalBackup_ConcurrentIncremental(t *testing.T) {
	routineState := model.RoutineState{
		Incremental: &model.RunningJob{
			StartTime: time.Now(),
		},
		LastRunTime: model.NewFullBackupTime(time.Now()),
	}

	backupCounters.Reset()

	routine := testRoutine()
	routine.BackupPolicy.ConcurrentIncremental = ptr.Of(true)
	runIncrementalBackup(t, routineState, routine)

	assert.Equal(t, 1, prometheusCounter(model.BackupJobTypeIncremental, BackupOutcomeSuccess))
}

func runIncrementalBackup(t *testing.T, state model.RoutineState, routine *model.BackupRoutine) {
	t.Helper()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClientManager := aerospike.NewMockClientManager(ctrl)

	mockBackupExecutor := backupexecutor.NewMockBackup(ctrl)
	mockBackupHandler := backupexecutor.NewMockBackupHandler(ctrl)
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockBackupBackend := NewMockBackupReaderWriter(ctrl)

	stats := models.NewBackupStats()
	stats.Start()
	stats.TotalRecords.Store(5)
	stats.IncFiles()
	stats.ReadRecords.Add(5)

	// Simulate an ongoing full backup
	mockRegistry.EXPECT().GetRoutineState(routine).Return(state).Times(1) // in createTimeBounds

	mockClientManager.EXPECT().GetClient(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockClient, nil).Times(2)
	mockClientManager.EXPECT().Close(mockClient).Times(2)

	mockBackupExecutor.EXPECT().Run(
		gomock.Any(),
		gomock.Any(),
		newTimeBoundsFromTimeMatcher(*state.LastRunTime.FullBackupTime()),
		gomock.Any(),
		gomock.Any(),
	).Return(mockBackupHandler, nil).Times(2)

	mockBackupHandler.EXPECT().GetStats().Return(stats).AnyTimes()
	mockBackupHandler.EXPECT().Wait(gomock.Any()).Return(nil).Times(2) // for ns1 and ns2

	mockRegistry.EXPECT().register(routineName, model.BackupJobTypeIncremental, gomock.Any())
	mockCompletionHandler.EXPECT().OnSuccess(gomock.Any(), routine, model.BackupJobTypeIncremental, gomock.Any(), gomock.Any())

	mockBackupBackend.EXPECT().WriteBackupMetadata(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).Times(2)

	startController := NewMockStartController(ctrl)
	startController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	nsExec := NewNamespaceBackupExecutor(mockClientManager, mockBackupExecutor, mockBackupBackend, NewPathService(nil))
	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, startController, nsExec)

	p.RunBackup(t.Context(), routine, time.Now(), model.BackupJobTypeIncremental)
	time.Sleep(10 * time.Millisecond) // time to unregister routine.

	assert.Equal(t, uint64(5), stats.TotalRecords.Load(), "Backup stats should be correct")
}

func prometheusCounter(jobType model.BackupJobType, outcome BackupOutcome) int {
	counter := backupCounters.WithLabelValues(routineName, string(jobType), string(outcome))
	return int(testutil.ToFloat64(counter))
}
