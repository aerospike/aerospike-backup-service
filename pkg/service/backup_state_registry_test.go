package service

import (
	"context"
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
		Timezone: model.NewServiceLocation(""),
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
	registry.SynchroniseBackupHistory(t.Context(), []*model.BackupRoutine{{Name: routineName}})

	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(backupStats).AnyTimes()
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	registry.BackupStarted(routineName, model.BackupTypeFull, handler)

	stat := registry.GetRoutineState(&model.BackupRoutine{
		Name:         routineName,
		IntervalCron: "@daily",
		Timezone:     model.NewServiceLocation(""),
	})
	assert.NotNil(t, stat.Full)
	assert.Equal(t, uint64(100), stat.Full.TotalRecords)
	assert.Nil(t, stat.Incremental)
	assert.Equal(t, stat.LastRunTime, backupTime)
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
		Timezone:     model.NewServiceLocation(""),
	}

	registry.BackupStarted(routineName, model.BackupTypeFull, handler)
	registry.getTracker(routineName).markScanDone()

	registry.BackupSucceeded(routine, model.BackupTypeFull)

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
		Timezone:     model.NewServiceLocation(""),
	}

	registry.BackupStarted(routineName, model.BackupTypeIncremental, handler)
	registry.getTracker(routineName).markScanDone()

	registry.BackupSucceeded(routine, model.BackupTypeIncremental)

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
		Timezone:     model.NewServiceLocation(""),
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
			Timezone:     model.NewServiceLocation(""),
		},
		routine2: {
			Name:         routine2,
			IntervalCron: "@daily",
			Timezone:     model.NewServiceLocation(""),
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
		Timezone:     model.NewRoutineLocation("America/New_York", model.NewServiceLocation("")),
	}
	utcRoutine := &model.BackupRoutine{
		Name:         "utc",
		IntervalCron: "@daily",
		Timezone:     model.NewServiceLocation(""),
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
		Timezone:         model.NewRoutineLocation("America/New_York", model.NewServiceLocation("")),
	}
	utcRoutine := &model.BackupRoutine{
		Name:             "utc",
		IntervalCron:     "@daily",
		IncrIntervalCron: "0 0 2 * * *",
		Timezone:         model.NewServiceLocation(""),
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
