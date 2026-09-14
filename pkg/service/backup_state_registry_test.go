package service

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newTestBackupStateRegistry(history HistoryManager, config routineProvider) *backupStateRegistry {
	return NewBackupStateRegistry(history, config).(*backupStateRegistry)
}

func TestRegisterAndCurrentStat(t *testing.T) {
	ctrl := gomock.NewController(t)

	registry := newTestBackupStateRegistry(nil, nil)
	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(backupStats).AnyTimes()
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	// Register a full backup handler
	registry.BackupStarted(routineName, model.BackupTypeFull, handler)
	registry.getTracker(routineName).markScanDone() // no need to scan history

	stat := registry.GetRoutineState(&model.BackupRoutine{
		Name:     routineName,
		Timezone: model.NewServiceLocation("", nil),
	})

	assert.NotNil(t, stat.Full)
	assert.Equal(t, uint64(100), stat.Full.TotalRecords)
	assert.Nil(t, stat.Incremental)
}

func TestHistoryScan(t *testing.T) {
	ctrl := gomock.NewController(t)

	historyMgr := NewMockHistoryManager(ctrl)
	backupTime := model.NewBackupTime(time.Now(), time.Now().Add(-1*time.Hour))
	historyMgr.EXPECT().FindLastRun(gomock.Any(), gomock.Any()).Return(backupTime, nil).Times(1)
	registry := newTestBackupStateRegistry(historyMgr, nil)
	registry.synchroniseBackupHistory(t.Context(), []*model.BackupRoutine{{Name: routineName}})

	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(backupStats).AnyTimes()
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	registry.BackupStarted(routineName, model.BackupTypeFull, handler)

	stat := registry.GetRoutineState(&model.BackupRoutine{
		Name:         routineName,
		IntervalCron: "@daily",
		Timezone:     model.NewServiceLocation("", nil),
	})
	assert.NotNil(t, stat.Full)
	assert.Equal(t, uint64(100), stat.Full.TotalRecords)
	assert.Nil(t, stat.Incremental)
	assert.Equal(t, stat.LastRunTime, backupTime)
}

// configWithRoutines returns a configuration holding one routine per given name. The
// registry resolves a queued name against this when it drains, so a request for a routine
// that is not here is a request for a routine that no longer exists.
func configWithRoutines(t *testing.T, names ...string) *model.Config {
	t.Helper()

	cfg := model.NewConfig()
	for _, name := range names {
		require.NoError(t, cfg.AddRoutine(&model.BackupRoutine{
			Name:         name,
			IntervalCron: "@daily",
			Timezone:     model.NewServiceLocation("", nil),
		}))
	}

	return cfg
}

func TestRequestHistorySync_WaitsForStart(t *testing.T) {
	ctrl := gomock.NewController(t)

	historyMgr := NewMockHistoryManager(ctrl)
	scanned := make(chan struct{}, 1)
	historyMgr.EXPECT().FindLastRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *model.BackupRoutine) (*model.BackupTime, error) {
			scanned <- struct{}{}
			return model.NewNoBackupTime(), nil
		}).Times(1)
	registry := newTestBackupStateRegistry(historyMgr, configWithRoutines(t, routineName))

	// Before Start there is no context to scan on, so a request only queues.
	registry.RequestHistorySync([]string{routineName})
	select {
	case <-scanned:
		t.Fatal("history was scanned before Start")
	case <-time.After(50 * time.Millisecond):
	}

	registry.Start(t.Context())
	waitAsyncDone(t, scanned, "history scan after Start")
}

func TestRequestHistorySync_CoalescesRequestsPerRoutine(t *testing.T) {
	cfg := configWithRoutines(t, routineName, "other")
	registry := newTestBackupStateRegistry(nil, cfg)

	registry.RequestHistorySync([]string{routineName, "other"})
	registry.RequestHistorySync([]string{routineName})

	pending := registry.takePending()
	require.Len(t, pending, 2, "a routine named twice is scanned once")
	assert.Equal(t, "other", pending[0].Name)
	assert.Same(t, cfg.Routines()[routineName], pending[1], "the scan uses the configured routine")
	assert.Empty(t, registry.takePending(), "taking drains the queue")
}

func TestRequestHistorySync_SkipsRoutineDeletedSinceTheRequest(t *testing.T) {
	registry := newTestBackupStateRegistry(nil, configWithRoutines(t, "survivor"))

	registry.RequestHistorySync([]string{"survivor", "deleted-since"})

	pending := registry.takePending()
	require.Len(t, pending, 1)
	assert.Equal(t, "survivor", pending[0].Name)
}

func TestRequestHistorySync_EmptyRequestQueuesNothing(t *testing.T) {
	registry := newTestBackupStateRegistry(nil, configWithRoutines(t))

	registry.RequestHistorySync(nil)

	assert.Empty(t, registry.takePending())
	select {
	case <-registry.signal:
		t.Fatal("an empty request must not signal a scan")
	default:
	}
}

func TestStart_IsIgnoredAndReportedWhenAlreadyRunning(t *testing.T) {
	originalLogger := slog.Default()
	logs := &logBuffer{}
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	ctrl := gomock.NewController(t)
	historyMgr := NewMockHistoryManager(ctrl)
	scanned := make(chan struct{}, 1)
	historyMgr.EXPECT().FindLastRun(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *model.BackupRoutine) (*model.BackupTime, error) {
			scanned <- struct{}{}
			return model.NewNoBackupTime(), nil
		}).Times(1)

	registry := newTestBackupStateRegistry(historyMgr, configWithRoutines(t, routineName))
	registry.Start(t.Context())

	rejected, cancelRejected := context.WithCancel(t.Context())
	registry.Start(rejected)
	cancelRejected()

	assert.Contains(t, logs.String(), "already running")

	// The lifetime the first Start gave still governs, so canceling the context that Start
	// refused stops nothing: a request made afterwards is still served.
	registry.RequestHistorySync([]string{routineName})
	waitAsyncDone(t, scanned, "history scan after a rejected Start")
}

func TestFinishFull(t *testing.T) {
	ctrl := gomock.NewController(t)

	now := time.Now()
	backupTime := model.NewFullBackupTime(now)

	historyMgr := NewMockHistoryManager(ctrl)
	historyMgr.EXPECT().FindLastRun(gomock.Any(), gomock.Any()).Return(backupTime, nil).Times(1)

	registry := newTestBackupStateRegistry(historyMgr, nil)
	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	routine := &model.BackupRoutine{
		Name:         routineName,
		IntervalCron: "@daily",
		Timezone:     model.NewServiceLocation("", nil),
	}

	registry.BackupStarted(routineName, model.BackupTypeFull, handler)
	registry.getTracker(routineName).markScanDone()

	registry.BackupSucceeded(t.Context(), routine, model.BackupTypeFull)

	stat := registry.GetRoutineState(routine)
	assert.Nil(t, stat.Full)
	assert.Nil(t, stat.Incremental)
	assert.Equal(t, now, *stat.LastRunTime.FullBackupTime())
}

func TestFinishIncremental(t *testing.T) {
	ctrl := gomock.NewController(t)

	now := time.Now()
	backupTime := model.NewBackupTime(now.Add(-1*time.Second), now)

	historyMgr := NewMockHistoryManager(ctrl)
	historyMgr.EXPECT().FindLastRun(gomock.Any(), gomock.Any()).Return(backupTime, nil).Times(1)

	registry := newTestBackupStateRegistry(historyMgr, nil)
	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	routine := &model.BackupRoutine{
		Name:         routineName,
		IntervalCron: "@daily",
		Timezone:     model.NewServiceLocation("", nil),
	}

	registry.BackupStarted(routineName, model.BackupTypeIncremental, handler)
	registry.getTracker(routineName).markScanDone()

	registry.BackupSucceeded(t.Context(), routine, model.BackupTypeIncremental)

	stat := registry.GetRoutineState(routine)
	assert.Nil(t, stat.Full)
	assert.Nil(t, stat.Incremental)
	assert.Equal(t, now, *stat.LastRunTime.IncrementalBackupTime())
}

func TestCanceledHistoryScanKeepsPreviousLastRun(t *testing.T) {
	ctrl := gomock.NewController(t)

	previous := model.NewFullBackupTime(time.Now())
	historyMgr := NewMockHistoryManager(ctrl)
	historyMgr.EXPECT().FindLastRun(gomock.Any(), gomock.Any()).Return(nil, context.Canceled).Times(1)

	registry := newTestBackupStateRegistry(historyMgr, nil)
	routine := &model.BackupRoutine{
		Name:         routineName,
		IntervalCron: "@daily",
		Timezone:     model.NewServiceLocation("", nil),
	}
	registry.getTracker(routineName).setLastRun(previous)
	registry.getTracker(routineName).markScanDone()

	err := registry.scanSingleRoutineHistory(t.Context(), routine)
	require.ErrorIs(t, err, context.Canceled)

	stat := registry.GetRoutineState(routine)
	assert.Equal(t, previous, stat.LastRunTime)
}

func TestGetAllCurrentStats(t *testing.T) {
	ctrl := gomock.NewController(t)

	routine1 := "routine1"
	routine2 := "routine2"

	mockReader := NewmockRoutineProvider(ctrl)
	mockReader.EXPECT().Routines().Return(map[string]*model.BackupRoutine{
		routine1: {
			Name:         routine1,
			IntervalCron: "@daily",
			Timezone:     model.NewServiceLocation("", nil),
		},
		routine2: {
			Name:         routine2,
			IntervalCron: "@daily",
			Timezone:     model.NewServiceLocation("", nil),
		},
	}).AnyTimes()

	registry := newTestBackupStateRegistry(nil, mockReader)
	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(backupStats).AnyTimes()
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	// Register handlers for multiple routines
	registry.BackupStarted(routine1, model.BackupTypeFull, handler)
	registry.getTracker(routine1).markScanDone() // no need to scan history
	registry.BackupStarted(routine2, model.BackupTypeIncremental, handler)
	registry.getTracker(routine2).markScanDone() // no need to scan history

	// Get all current stats
	stats := registry.GetRunningState()
	assert.Len(t, stats, 2)

	// Check stats for routine1
	assert.NotNil(t, stats[routine1].Full)
	assert.Nil(t, stats[routine1].Incremental)

	// Check stats for routine2
	assert.Nil(t, stats[routine2].Full)
	assert.NotNil(t, stats[routine2].Incremental)
}

func TestGetRoutineState_NextRunTimeUsesScheduleTimezone(t *testing.T) {
	t.Parallel()

	nyRoutine := &model.BackupRoutine{
		Name:         "ny",
		IntervalCron: "@daily",
		Timezone:     testLocation,
	}
	utcRoutine := &model.BackupRoutine{
		Name:         "utc",
		IntervalCron: "@daily",
		Timezone:     model.NewServiceLocation("", nil),
	}

	registry := newTestBackupStateRegistry(nil, nil)
	registry.getTracker(nyRoutine.Name).markScanDone()
	registry.getTracker(utcRoutine.Name).markScanDone()

	nyNext := registry.GetRoutineState(nyRoutine).NextRunTime.FullBackupTime()
	utcNext := registry.GetRoutineState(utcRoutine).NextRunTime.FullBackupTime()
	require.NotNil(t, nyNext)
	require.NotNil(t, utcNext)

	nyLoc := nyRoutine.Timezone.ResolvedLocation()
	assert.Equal(t, 0, nyNext.In(nyLoc).Hour())
	assert.Equal(t, 0, nyNext.In(nyLoc).Minute())
	assert.Equal(t, 0, utcNext.In(model.DefaultScheduleTimezone).Hour())
	assert.NotEqual(t, nyNext.UTC(), utcNext.UTC())
}

func TestGetRoutineState_NextRunTimeUsesScheduleTimezoneForIncremental(t *testing.T) {
	t.Parallel()

	nyRoutine := &model.BackupRoutine{
		Name:             "ny",
		IntervalCron:     "@daily",
		IncrIntervalCron: "0 0 2 * * *",
		Timezone:         testLocation,
	}
	utcRoutine := &model.BackupRoutine{
		Name:             "utc",
		IntervalCron:     "@daily",
		IncrIntervalCron: "0 0 2 * * *",
		Timezone:         model.NewServiceLocation("", nil),
	}

	registry := newTestBackupStateRegistry(nil, nil)
	registry.getTracker(nyRoutine.Name).markScanDone()
	registry.getTracker(utcRoutine.Name).markScanDone()

	nyNext := registry.GetRoutineState(nyRoutine).NextRunTime.IncrementalBackupTime()
	utcNext := registry.GetRoutineState(utcRoutine).NextRunTime.IncrementalBackupTime()
	require.NotNil(t, nyNext)
	require.NotNil(t, utcNext)

	nyLoc := nyRoutine.Timezone.ResolvedLocation()
	assert.Equal(t, 2, nyNext.In(nyLoc).Hour())
	assert.Equal(t, 2, utcNext.In(model.DefaultScheduleTimezone).Hour())
	assert.NotEqual(t, nyNext.UTC(), utcNext.UTC())
}

func TestCancel(t *testing.T) {
	ctrl := gomock.NewController(t)

	registry := newTestBackupStateRegistry(nil, nil)
	handlerFull := NewMockCancelableBackupHandler(ctrl)
	handlerFull.EXPECT().Cancel()

	handlerIncr := NewMockCancelableBackupHandler(ctrl)
	handlerIncr.EXPECT().Cancel()

	registry.BackupStarted(routineName, model.BackupTypeFull, handlerFull)
	registry.BackupStarted(routineName, model.BackupTypeIncremental, handlerIncr)

	registry.Cancel(routineName)
}
