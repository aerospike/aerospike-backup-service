package service

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
)

type mockCancelableBackupHandler struct {
	stats    *models.BackupStats
	canceled bool
}

func (m *mockCancelableBackupHandler) GetStats() *models.BackupStats {
	return m.stats
}

func (m *mockCancelableBackupHandler) Wait(context.Context) error {
	return nil
}

func (m *mockCancelableBackupHandler) Cancel() {
	m.canceled = true
}

func (m *mockCancelableBackupHandler) IsCanceled() bool {
	return m.canceled
}

func TestRegisterAndCurrentStat(t *testing.T) {
	registry := NewRunningBackupsRegistry(context.Background())

	routineName := "routine1"
	backupStats := models.NewBackupStats()
	backupStats.TotalRecords = 100

	handler := &mockCancelableBackupHandler{
		stats: backupStats,
	}

	// Register a full backup handler
	registry.register(routineName, jobTypeFull, handler)

	stat := registry.GetRoutineState(routineName)
	assert.Equal(t, stat.Full.TotalRecords, uint64(100))
	assert.Nil(t, stat.Incremental)
}

func TestFinishFull(t *testing.T) {
	registry := NewRunningBackupsRegistry(context.Background())

	routineName := "routine1"
	handler := &mockCancelableBackupHandler{}

	registry.register(routineName, jobTypeFull, handler)

	now := time.Now()
	registry.unregister(routineName, jobTypeFull, now)

	stat := registry.GetRoutineState(routineName)
	assert.Nil(t, stat.Full)
	assert.Nil(t, stat.Incremental)
	assert.Equal(t, now, *stat.LastRunTime.FullBackupTime())
}

func TestFinishIncremental(t *testing.T) {
	registry := NewRunningBackupsRegistry(context.Background())

	routineName := "routine1"
	handler := &mockCancelableBackupHandler{}

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
	registry := NewRunningBackupsRegistry(context.Background())

	routine1 := "routine1"
	routine2 := "routine2"
	backupStats := models.NewBackupStats()
	backupStats.TotalRecords = 100

	handler := &mockCancelableBackupHandler{
		stats: backupStats,
	}

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
	registry := NewRunningBackupsRegistry(context.Background())

	routineName := "routine1"
	handlerFull := &mockCancelableBackupHandler{}
	handlerIncr := &mockCancelableBackupHandler{}

	registry.register(routineName, jobTypeFull, handlerFull)
	registry.register(routineName, jobTypeIncremental, handlerIncr)

	registry.Cancel(routineName)

	assert.True(t, handlerFull.canceled)
	assert.True(t, handlerIncr.canceled)
}
