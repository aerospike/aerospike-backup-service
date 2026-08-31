package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/optional"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go/models"
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
		Timezone:     time.UTC,
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

	routine := testRoutine()
	now := time.Now()

	mockRegistry := NewMockBackupStateRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockReporter := NewMockBackupReporter(ctrl)
	mockRunner := NewMockRoutineBackupRunner(ctrl)

	stats := newTestBackupStats(10)
	op := newBackupNamespacesOperation(ctrl, routine.Namespaces, stats)

	mockRunner.EXPECT().Run(gomock.Any(), routine, gomock.Any(), gomock.Any()).Return(op, nil)
	mockRegistry.EXPECT().BackupStarted(routineName, model.BackupTypeFull, gomock.Any())
	mockCompletionHandler.EXPECT().
		OnSuccess(gomock.Any(), routine, model.BackupTypeFull, newTimeMatcher(now), gomock.Any())
	mockReporter.EXPECT().
		Report(routine.Name, model.BackupTypeFull, newTimeMatcher(now), gomock.Any(), nil, gomock.Any())

	mockStartController := NewMockStartController(ctrl)
	mockStartController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, mockReporter, mockStartController, mockRunner)

	p.Backup(t.Context(), routine, now, model.BackupTypeFull)

	assert.Equal(t, uint64(10), stats.TotalRecords.Load(), "Backup stats should be correct")
}

func TestRunFullBackupInternal_SkipWhenBackupInProgress(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := testRoutine()
	now := time.Now()

	mockRegistry := NewMockBackupStateRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockReporter := NewMockBackupReporter(ctrl)

	skipReason := errors.New("backup already running")
	mockStartController := NewMockStartController(ctrl)
	mockStartController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, skipReason)

	mockReporter.EXPECT().
		Report(routine.Name, model.BackupTypeFull, newTimeMatcher(now), gomock.Any(), skipReason, gomock.Any())

	mockRunner := NewMockRoutineBackupRunner(ctrl)
	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, mockReporter, mockStartController, mockRunner)

	p.Backup(t.Context(), routine, now, model.BackupTypeFull)
}

func TestRunFullBackupInternal_ClientConnectionFailure(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := testRoutine()
	now := time.Now()

	mockRegistry := NewMockBackupStateRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockReporter := NewMockBackupReporter(ctrl)

	connectionError := errors.New("connection failed")
	mockRunner := NewMockRoutineBackupRunner(ctrl)
	mockRunner.EXPECT().Run(gomock.Any(), routine, gomock.Any(), gomock.Any()).Return(nil, connectionError)

	mockStartController := NewMockStartController(ctrl)
	mockStartController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	mockReporter.EXPECT().
		Report(routine.Name, model.BackupTypeFull, newTimeMatcher(now), gomock.Any(), connectionError, gomock.Any())

	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, mockReporter, mockStartController, mockRunner)

	p.Backup(t.Context(), routine, now, model.BackupTypeFull)
}

func TestRunFullBackupInternal_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := testRoutine()
	now := time.Now()

	mockRegistry := NewMockBackupStateRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockReporter := NewMockBackupReporter(ctrl)

	mockRunner := NewMockRoutineBackupRunner(ctrl)
	mockRunner.EXPECT().Run(gomock.Any(), routine, gomock.Any(), gomock.Any()).Return(nil, context.Canceled)

	mockStartController := NewMockStartController(ctrl)
	mockStartController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	mockReporter.EXPECT().
		Report(routine.Name, model.BackupTypeFull, newTimeMatcher(now), gomock.Any(), context.Canceled, gomock.Any())

	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, mockReporter, mockStartController, mockRunner)

	p.Backup(t.Context(), routine, now, model.BackupTypeFull)
}

func TestRunIncrementalBackup_Success(t *testing.T) {
	from := time.Unix(1700000000, 0)
	routineState := model.RoutineState{
		LastRunTime: model.NewFullBackupTime(from),
	}

	runIncrementalBackupSuccess(t, routineState, testRoutine())
}

func TestRunIncrementalBackup_Skip(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := testRoutine()
	now := time.Now()
	mockRegistry := NewMockBackupStateRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockReporter := NewMockBackupReporter(ctrl)

	skipReason := errors.New("no previous full backup")
	mockStartController := NewMockStartController(ctrl)
	mockStartController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, skipReason)

	mockReporter.EXPECT().
		Report(routine.Name, model.BackupTypeIncremental, newTimeMatcher(now), gomock.Any(), skipReason, gomock.Any())

	mockRunner := NewMockRoutineBackupRunner(ctrl)
	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, mockReporter, mockStartController, mockRunner)

	p.Backup(t.Context(), routine, now, model.BackupTypeIncremental)
}

func TestRunIncrementalBackup_ContextCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine := testRoutine()
	now := time.Now()
	from := time.Unix(1700000001, 0)
	state := model.RoutineState{LastRunTime: model.NewFullBackupTime(from)}

	mockRegistry := NewMockBackupStateRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockReporter := NewMockBackupReporter(ctrl)

	mockRegistry.EXPECT().GetRoutineState(routine).Return(state).Times(1)

	mockRunner := NewMockRoutineBackupRunner(ctrl)
	mockRunner.EXPECT().Run(gomock.Any(), routine, gomock.Any(), gomock.Any()).
		Return(nil, context.DeadlineExceeded)

	mockStartController := NewMockStartController(ctrl)
	mockStartController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	mockReporter.EXPECT().Report(
		routine.Name, model.BackupTypeIncremental, newTimeMatcher(now), gomock.Any(),
		context.DeadlineExceeded, gomock.Any(),
	)

	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, mockReporter, mockStartController, mockRunner)

	p.Backup(t.Context(), routine, now, model.BackupTypeIncremental)
}

func TestRunIncrementalBackup_AllowConcurrentFull(t *testing.T) {
	from := time.Unix(1700000002, 0)
	routineState := model.RoutineState{
		Full:        &model.RunningJob{StartTime: time.Now()},
		LastRunTime: model.NewFullBackupTime(from),
	}

	routine := testRoutine()
	routine.BackupPolicy.ConcurrentIncremental = ptr.Of(true)
	runIncrementalBackupSuccess(t, routineState, routine)
}

func TestRunIncrementalBackup_ConcurrentIncremental(t *testing.T) {
	from := time.Unix(1700000003, 0)
	routineState := model.RoutineState{
		Incremental: &model.RunningJob{StartTime: time.Now()},
		LastRunTime: model.NewFullBackupTime(from),
	}

	routine := testRoutine()
	routine.BackupPolicy.ConcurrentIncremental = ptr.Of(true)
	runIncrementalBackupSuccess(t, routineState, routine)
}

func runIncrementalBackupSuccess(t *testing.T, state model.RoutineState, routine *model.BackupRoutine) {
	t.Helper()
	ctrl := gomock.NewController(t)

	now := time.Now()
	fromTime := *state.LastRunTime.LatestRun()

	mockRegistry := NewMockBackupStateRegistry(ctrl)
	mockCompletionHandler := NewMockBackupCompletionHandler(ctrl)
	mockReporter := NewMockBackupReporter(ctrl)
	mockRunner := NewMockRoutineBackupRunner(ctrl)

	stats := newTestBackupStats(5)
	op := newBackupNamespacesOperation(ctrl, routine.Namespaces, stats)

	mockRegistry.EXPECT().GetRoutineState(routine).Return(state).Times(1)
	mockRunner.EXPECT().Run(
		gomock.Any(),
		routine,
		gomock.AssignableToTypeOf(model.BackupRunSpec{}),
		gomock.Any(),
	).Do(func(_ context.Context, _ *model.BackupRoutine, spec model.BackupRunSpec, _ *slog.Logger) {
		assert.Equal(t, model.BackupTypeIncremental, spec.Type)
		assert.NotNil(t, spec.TimeBounds.FromTime)
		assert.Equal(t, fromTime, *spec.TimeBounds.FromTime)
	}).Return(op, nil)

	mockRegistry.EXPECT().BackupStarted(routineName, model.BackupTypeIncremental, gomock.Any())
	mockCompletionHandler.EXPECT().OnSuccess(
		gomock.Any(),
		routine,
		model.BackupTypeIncremental,
		gomock.Any(),
		gomock.Any(),
	)
	mockReporter.EXPECT().
		Report(routine.Name, model.BackupTypeIncremental, newTimeMatcher(now), gomock.Any(), nil, gomock.Any())

	startController := NewMockStartController(ctrl)
	startController.EXPECT().TryStart(gomock.Any(), gomock.Any(), gomock.Any()).Return(func() {}, nil)

	p := NewBackupOrchestrator(mockRegistry, mockCompletionHandler, mockReporter, startController, mockRunner)

	p.Backup(t.Context(), routine, now, model.BackupTypeIncremental)

	assert.Equal(t, uint64(5), stats.TotalRecords.Load(), "Backup stats should be correct")
}
