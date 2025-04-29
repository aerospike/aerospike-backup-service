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
	registry := NewRunningBackupsRegistry(context.Background(), &MockBackupBackendService{}, model.NewConfig())

	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(backupStats).AnyTimes()
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	// Register a full backup handler
	registry.register(routineName, jobTypeFull, handler)

	stat := registry.GetRoutineState(routineName)
	assert.Equal(t, stat.Full.TotalRecords, uint64(100))
	assert.Nil(t, stat.Incremental)
}

func TestFinishFull(t *testing.T) {
	registry := NewRunningBackupsRegistry(context.Background(), &MockBackupBackendService{}, model.NewConfig())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	registry.register(routineName, jobTypeFull, handler)

	now := time.Now()
	registry.unregister(routineName, jobTypeFull, now)

	stat := registry.GetRoutineState(routineName)
	assert.Nil(t, stat.Full)
	assert.Nil(t, stat.Incremental)
	assert.Equal(t, now, *stat.LastRunTime.FullBackupTime())
}

func TestFinishIncremental(t *testing.T) {
	registry := NewRunningBackupsRegistry(context.Background(), &MockBackupBackendService{}, model.NewConfig())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handler := NewMockCancelableBackupHandler(ctrl)
	// handler.EXPECT().GetStats().Return(backupStats).AnyTimes()
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	registry.register(routineName, jobTypeIncremental, handler)

	now := time.Now()
	registry.unregister(routineName, jobTypeFull, now.Add(-1*time.Second))
	registry.unregister(routineName, jobTypeIncremental, now)

	stat := registry.GetRoutineState(routineName)
	assert.Nil(t, stat.Full)
	assert.Nil(t, stat.Incremental)
	assert.Equal(t, now, *stat.LastRunTime.IncrementalBackupTime())
}

func TestGetAllCurrentStats(t *testing.T) {
	registry := NewRunningBackupsRegistry(context.Background(), &MockBackupBackendService{}, model.NewConfig())

	routine1 := "routine1"
	routine2 := "routine2"
	backupStats := models.NewBackupStats()
	backupStats.TotalRecords.Store(100)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	handler := NewMockCancelableBackupHandler(ctrl)
	handler.EXPECT().GetStats().Return(backupStats).AnyTimes()
	handler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	// Register handlers for multiple routines
	registry.register(routine1, jobTypeFull, handler)
	registry.register(routine2, jobTypeIncremental, handler)

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
	registry := NewRunningBackupsRegistry(context.Background(), &MockBackupBackendService{}, model.NewConfig())

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
