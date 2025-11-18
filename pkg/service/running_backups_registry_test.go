package service

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestRegisterAndCurrentStat(t *testing.T) {
	registry := NewRunningBackupsRegistry(nil, initConfig())

	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(backupStats).AnyTimes()
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	// Register a full backup handler
	registry.register(routineName, jobTypeFull, handler)
	registry.getTracker(routineName).signalSyncDone() // no need to scan history

	stat := registry.GetRoutineState(routineName)
	assert.NotNil(t, stat.Full)
	assert.Equal(t, uint64(100), stat.Full.TotalRecords)
	assert.Nil(t, stat.Incremental)
}

func TestHistoryScan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	historyMgr := NewMockHistoryManager(ctrl)
	backupTime := model.NewBackupTime(time.Now(), time.Now().Add(-1*time.Hour))
	historyMgr.EXPECT().FindLastRun(gomock.Any(), routineName).Return(backupTime, nil)

	registry := NewRunningBackupsRegistry(historyMgr, initConfig())
	registry.SynchroniseBackupHistory(context.Background())

	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(backupStats).AnyTimes()
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	registry.register(routineName, jobTypeFull, handler)

	stat := registry.GetRoutineState(routineName)
	assert.NotNil(t, stat.Full)
	assert.Equal(t, uint64(100), stat.Full.TotalRecords)
	assert.Nil(t, stat.Incremental)
	assert.Equal(t, stat.LastRunTime, backupTime)
}

func TestFinishFull(t *testing.T) {
	registry := NewRunningBackupsRegistry(nil, initConfig())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	registry.register(routineName, jobTypeFull, handler)
	registry.getTracker(routineName).signalSyncDone() // no need to scan history

	now := time.Now()
	registry.recordSuccessfulBackup(routineName, jobTypeFull, now)

	stat := registry.GetRoutineState(routineName)
	assert.Nil(t, stat.Full)
	assert.Nil(t, stat.Incremental)
	assert.Equal(t, now, *stat.LastRunTime.FullBackupTime())
}

func TestFinishIncremental(t *testing.T) {
	registry := NewRunningBackupsRegistry(nil, initConfig())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	registry.register(routineName, jobTypeIncremental, handler)
	registry.getTracker(routineName).signalSyncDone() // no need to scan history

	now := time.Now()
	registry.recordSuccessfulBackup(routineName, jobTypeFull, now.Add(-1*time.Second))
	registry.recordSuccessfulBackup(routineName, jobTypeIncremental, now)

	stat := registry.GetRoutineState(routineName)
	assert.Nil(t, stat.Full)
	assert.Nil(t, stat.Incremental)
	assert.Equal(t, now, *stat.LastRunTime.IncrementalBackupTime())
}

func TestGetAllCurrentStats(t *testing.T) {
	config := initConfig()
	registry := NewRunningBackupsRegistry(nil, config)

	routine1 := "routine1"
	routine2 := "routine2"
	_ = config.AddRoutine(routine1, &model.BackupRoutine{
		IntervalCron: "@daily",
	})
	_ = config.AddRoutine(routine2, &model.BackupRoutine{
		IntervalCron: "@daily",
	})

	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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
	registry := NewRunningBackupsRegistry(nil, initConfig())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handlerFull := NewMockCancelableBackupHandler(ctrl)
	handlerFull.EXPECT().Cancel()

	handlerIncr := NewMockCancelableBackupHandler(ctrl)
	handlerIncr.EXPECT().Cancel()

	registry.register(routineName, jobTypeFull, handlerFull)
	registry.register(routineName, jobTypeIncremental, handlerIncr)

	registry.Cancel(routineName)
}

func initConfig() *model.Config {
	config := model.NewConfig()
	_ = config.AddRoutine(routineName, &model.BackupRoutine{
		IntervalCron: "@daily",
	})
	return config
}
