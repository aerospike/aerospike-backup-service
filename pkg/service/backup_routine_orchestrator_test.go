package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
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

func newTestBackupStats(total uint64) *models.BackupStats {
	stats := models.NewBackupStats()
	stats.Start()
	stats.TotalRecords.Store(total)
	stats.IncFiles()
	stats.ReadRecords.Add(total)
	return stats
}

// newBackupNamespacesOperation returns an aggregate op backed by one mock handler shared
// across namespaces (same pattern as production: one Wait per namespace).
func newBackupNamespacesOperation(
	ctrl *gomock.Controller,
	namespaces []string,
	stats *models.BackupStats,
) *BackupNamespacesOperation {
	h := NewMockCancelableBackupHandler(ctrl)
	h.EXPECT().Wait(gomock.Any()).Return(nil).Times(len(namespaces))
	if stats != nil {
		h.EXPECT().GetStats().Return(stats).AnyTimes()
	}
	handlers := make(map[string]CancelableBackupHandler, len(namespaces))
	for _, ns := range namespaces {
		handlers[ns] = h
	}
	return &BackupNamespacesOperation{handlers: handlers}
}

func TestRunFullBackupInternal_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := testRoutine()
	now := time.Now()

	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockRunner := NewMockAllNamespacesBackupRunner(ctrl)

	stats := newTestBackupStats(10)
	op := newBackupNamespacesOperation(ctrl, routine.Namespaces, stats)

	mockRunner.EXPECT().StartBackup(gomock.Any(), gomock.Any(), routine, gomock.Any()).Return(op, nil)
	mockRegistry.EXPECT().register(routineName, model.BackupTypeFull, gomock.Any())
	mockCompletionHandler.EXPECT().
		OnSuccess(gomock.Any(), routine, model.BackupTypeFull, newTimeMatcher(now), gomock.Any())

	mockStartController := NewMockStartController(ctrl)
	mockStartController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, mockStartController, mockRunner)

	backupCounters.Reset()
	p.RunBackup(t.Context(), routine, now, model.BackupTypeFull)

	assert.Equal(t, uint64(10), stats.TotalRecords.Load(), "Backup stats should be correct")
	assert.Equal(t, 1, prometheusCounter(model.BackupTypeFull, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(model.BackupTypeFull, BackupOutcomeSkip))
	assert.Zero(t, prometheusCounter(model.BackupTypeFull, BackupOutcomeFailure))
}

func TestRunFullBackupInternal_SkipWhenBackupInProgress(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := testRoutine()

	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)

	mockStartController := NewMockStartController(ctrl)
	mockStartController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, errBackupSkipped)

	mockRunner := NewMockAllNamespacesBackupRunner(ctrl)
	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, mockStartController, mockRunner)

	backupCounters.Reset()

	p.RunBackup(t.Context(), routine, time.Now(), model.BackupTypeFull)

	assert.Zero(t, prometheusCounter(model.BackupTypeFull, BackupOutcomeSuccess))
	assert.Equal(t, 1, prometheusCounter(model.BackupTypeFull, BackupOutcomeSkip))
	assert.Zero(t, prometheusCounter(model.BackupTypeFull, BackupOutcomeFailure))
}

func TestRunFullBackupInternal_ClientConnectionFailure(t *testing.T) {
	defer func(old *slog.Logger) {
		slog.SetDefault(old)
	}(slog.Default())

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := testRoutine()

	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)

	connectionError := errors.New("connection failed")
	mockRunner := NewMockAllNamespacesBackupRunner(ctrl)
	mockRunner.EXPECT().StartBackup(gomock.Any(), gomock.Any(), routine, gomock.Any()).Return(nil, connectionError)

	mockStartController := NewMockStartController(ctrl)
	mockStartController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, mockStartController, mockRunner)

	backupCounters.Reset()
	p.RunBackup(t.Context(), routine, time.Now(), model.BackupTypeFull)

	assert.Contains(t, buf.String(), connectionError.Error())

	assert.Zero(t, prometheusCounter(model.BackupTypeFull, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(model.BackupTypeFull, BackupOutcomeSkip))
	assert.Equal(t, 1, prometheusCounter(model.BackupTypeFull, BackupOutcomeFailure))
}

func TestRunFullBackupInternal_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := testRoutine()

	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)

	mockRunner := NewMockAllNamespacesBackupRunner(ctrl)
	mockRunner.EXPECT().StartBackup(gomock.Any(), gomock.Any(), routine, gomock.Any()).Return(nil, context.Canceled)

	mockStartController := NewMockStartController(ctrl)
	mockStartController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, mockStartController, mockRunner)

	backupCounters.Reset()
	p.RunBackup(t.Context(), routine, time.Now(), model.BackupTypeFull)

	assert.Zero(t, prometheusCounter(model.BackupTypeFull, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(model.BackupTypeFull, BackupOutcomeSkip))
	assert.Zero(t, prometheusCounter(model.BackupTypeFull, BackupOutcomeFailure))
	assert.Equal(t, 1, prometheusCounter(model.BackupTypeFull, BackupOutcomeCanceled))
}

func TestRunIncrementalBackup_Success(t *testing.T) {
	from := time.Unix(1700000000, 0)
	routineState := model.RoutineState{
		LastRunTime: model.NewFullBackupTime(from),
	}

	backupCounters.Reset()

	runIncrementalBackupSuccess(t, routineState, testRoutine())

	assert.Equal(t, 1, prometheusCounter(model.BackupTypeIncremental, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(model.BackupTypeIncremental, BackupOutcomeFailure))
	assert.Zero(t, prometheusCounter(model.BackupTypeIncremental, BackupOutcomeSkip))
}

func TestRunIncrementalBackup_Skip(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := testRoutine()
	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)

	mockStartController := NewMockStartController(ctrl)
	mockStartController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, errBackupSkipped)

	mockRunner := NewMockAllNamespacesBackupRunner(ctrl)
	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, mockStartController, mockRunner)

	backupCounters.Reset()

	p.RunBackup(t.Context(), routine, time.Now(), model.BackupTypeIncremental)

	assert.Equal(t, 1, prometheusCounter(model.BackupTypeIncremental, BackupOutcomeSkip))
	assert.Zero(t, prometheusCounter(model.BackupTypeIncremental, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(model.BackupTypeIncremental, BackupOutcomeFailure))
}

func TestRunIncrementalBackup_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine := testRoutine()
	from := time.Unix(1700000001, 0)
	state := model.RoutineState{LastRunTime: model.NewFullBackupTime(from)}

	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)

	mockRegistry.EXPECT().GetRoutineState(routine).Return(state).Times(1)

	mockRunner := NewMockAllNamespacesBackupRunner(ctrl)
	mockRunner.EXPECT().StartBackup(gomock.Any(), gomock.Any(), routine, gomock.Any()).
		Return(nil, context.DeadlineExceeded)

	mockStartController := NewMockStartController(ctrl)
	mockStartController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, mockStartController, mockRunner)

	backupCounters.Reset()
	p.RunBackup(t.Context(), routine, time.Now(), model.BackupTypeIncremental)

	assert.Zero(t, prometheusCounter(model.BackupTypeIncremental, BackupOutcomeSuccess))
	assert.Zero(t, prometheusCounter(model.BackupTypeIncremental, BackupOutcomeSkip))
	assert.Zero(t, prometheusCounter(model.BackupTypeIncremental, BackupOutcomeFailure))
	assert.Equal(t, 1, prometheusCounter(model.BackupTypeIncremental, BackupOutcomeCanceled))
}

func TestRunIncrementalBackup_AllowConcurrentFull(t *testing.T) {
	from := time.Unix(1700000002, 0)
	routineState := model.RoutineState{
		Full:        &model.RunningJob{StartTime: time.Now()},
		LastRunTime: model.NewFullBackupTime(from),
	}

	backupCounters.Reset()

	routine := testRoutine()
	routine.BackupPolicy.ConcurrentIncremental = ptr.Of(true)
	runIncrementalBackupSuccess(t, routineState, routine)

	assert.Equal(t, 1, prometheusCounter(model.BackupTypeIncremental, BackupOutcomeSuccess))
}

func TestRunIncrementalBackup_ConcurrentIncremental(t *testing.T) {
	from := time.Unix(1700000003, 0)
	routineState := model.RoutineState{
		Incremental: &model.RunningJob{StartTime: time.Now()},
		LastRunTime: model.NewFullBackupTime(from),
	}

	backupCounters.Reset()

	routine := testRoutine()
	routine.BackupPolicy.ConcurrentIncremental = ptr.Of(true)
	runIncrementalBackupSuccess(t, routineState, routine)

	assert.Equal(t, 1, prometheusCounter(model.BackupTypeIncremental, BackupOutcomeSuccess))
}

func runIncrementalBackupSuccess(t *testing.T, state model.RoutineState, routine *model.BackupRoutine) {
	t.Helper()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fromTime := *state.LastRunTime.LatestRun()

	mockRegistry := NewMockRunningBackupsRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockRunner := NewMockAllNamespacesBackupRunner(ctrl)

	stats := newTestBackupStats(5)
	op := newBackupNamespacesOperation(ctrl, routine.Namespaces, stats)

	mockRegistry.EXPECT().GetRoutineState(routine).Return(state).Times(1)
	mockRunner.EXPECT().StartBackup(
		gomock.Any(),
		gomock.Any(),
		routine,
		gomock.AssignableToTypeOf(model.BackupRunSpec{}),
	).Do(func(_ context.Context, _ *slog.Logger, _ *model.BackupRoutine, spec model.BackupRunSpec) {
		assert.Equal(t, model.BackupTypeIncremental, spec.Type)
		assert.NotNil(t, spec.TimeBounds.FromTime)
		assert.Equal(t, fromTime, *spec.TimeBounds.FromTime)
	}).Return(op, nil)

	mockRegistry.EXPECT().register(routineName, model.BackupTypeIncremental, gomock.Any())
	mockCompletionHandler.EXPECT().OnSuccess(
		gomock.Any(),
		routine,
		model.BackupTypeIncremental,
		gomock.Any(),
		gomock.Any(),
	)

	startController := NewMockStartController(ctrl)
	startController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, startController, mockRunner)

	p.RunBackup(t.Context(), routine, time.Now(), model.BackupTypeIncremental)

	assert.Equal(t, uint64(5), stats.TotalRecords.Load(), "Backup stats should be correct")
}

func prometheusCounter(jobType model.BackupType, outcome BackupOutcome) int {
	counter := backupCounters.WithLabelValues(routineName, string(jobType), string(outcome))
	return int(testutil.ToFloat64(counter))
}
