package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestRegisterAndCurrentStat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	registry := NewRunningBackupsRegistry(nil, nil)

	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(backupStats).AnyTimes()
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	// Register a full backup handler
	registry.register(routineName, jobTypeFull, handler)
	registry.getTracker(routineName).signalSyncDone() // no need to scan history

	stat := registry.GetRoutineState(&model.BackupRoutine{
		Name: routineName,
	})

	assert.NotNil(t, stat.Full)
	assert.Equal(t, uint64(100), stat.Full.TotalRecords)
	assert.Nil(t, stat.Incremental)
}

func TestHistoryScan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	historyMgr := NewMockHistoryManager(ctrl)
	backupTime := model.NewBackupTime(time.Now(), time.Now().Add(-1*time.Hour))
	historyMgr.EXPECT().FindLastRun(gomock.Any(), gomock.Any()).Return(backupTime, nil).Times(1)

	mockReader := NewMockroutineProvider(ctrl)
	mockReader.EXPECT().PopInvalidatedRoutines().Return([]*model.BackupRoutine{{Name: routineName}}).Times(1)

	registry := NewRunningBackupsRegistry(historyMgr, mockReader)
	registry.SynchroniseBackupHistory(t.Context())

	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(backupStats).AnyTimes()
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	registry.register(routineName, jobTypeFull, handler)

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
	defer ctrl.Finish()

	registry := NewRunningBackupsRegistry(nil, nil)

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	registry.register(routineName, jobTypeFull, handler)
	registry.getTracker(routineName).signalSyncDone() // no need to scan history

	now := time.Now()
	registry.recordSuccessfulBackup(routineName, jobTypeFull, now)

	routine := &model.BackupRoutine{
		Name:         routineName,
		IntervalCron: "@daily",
	}

	stat := registry.GetRoutineState(routine)
	assert.Nil(t, stat.Full)
	assert.Nil(t, stat.Incremental)
	assert.Equal(t, now, *stat.LastRunTime.FullBackupTime())
}

func TestFinishIncremental(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	registry := NewRunningBackupsRegistry(nil, nil)

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	registry.register(routineName, jobTypeIncremental, handler)
	registry.getTracker(routineName).signalSyncDone() // no need to scan history

	now := time.Now()
	registry.recordSuccessfulBackup(routineName, jobTypeFull, now.Add(-1*time.Second))
	registry.recordSuccessfulBackup(routineName, jobTypeIncremental, now)

	routine := &model.BackupRoutine{
		Name:         routineName,
		IntervalCron: "@daily",
	}

	stat := registry.GetRoutineState(routine)
	assert.Nil(t, stat.Full)
	assert.Nil(t, stat.Incremental)
	assert.Equal(t, now, *stat.LastRunTime.IncrementalBackupTime())
}

func TestGetAllCurrentStats(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	routine1 := "routine1"
	routine2 := "routine2"

	mockReader := NewMockroutineProvider(ctrl)
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

	registry := NewRunningBackupsRegistry(nil, mockReader)

	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(backupStats).AnyTimes()
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	// Register handlers for multiple routines
	registry.register(routine1, jobTypeFull, handler)
	registry.getTracker(routine1).signalSyncDone() // no need to scan history
	registry.register(routine2, jobTypeIncremental, handler)
	registry.getTracker(routine2).signalSyncDone() // no need to scan history

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
	defer ctrl.Finish()

	registry := NewRunningBackupsRegistry(nil, nil)

	handlerFull := NewMockCancelableBackupHandler(ctrl)
	handlerFull.EXPECT().Cancel()

	handlerIncr := NewMockCancelableBackupHandler(ctrl)
	handlerIncr.EXPECT().Cancel()

	registry.register(routineName, jobTypeFull, handlerFull)
	registry.register(routineName, jobTypeIncremental, handlerIncr)

	registry.Cancel(routineName)
}
