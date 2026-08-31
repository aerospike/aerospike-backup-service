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
		Name: routineName,
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
		},
		routine2: {
			Name:         routine2,
			IntervalCron: "@daily",
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
