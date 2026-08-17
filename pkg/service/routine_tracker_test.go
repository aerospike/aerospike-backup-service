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

func TestNewRoutineTracker(t *testing.T) {
	tracker := newRoutineTracker()
	assert.NotNil(t, tracker)
	assert.NotNil(t, tracker.lastRun)
	assert.NotNil(t, tracker.scanDone)
	// scanDone should be open (blocking until first scan)
	select {
	case <-tracker.scanDone:
		t.Fatal("scanDone should be open")
	default:
		// success
	}
}

func TestGetState_BlockingAndTimeout(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()

	// Test timeout
	snapshot, err := tracker.getState(10 * time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Nil(t, snapshot)

	// Test blocking and unblocking
	go func() {
		time.Sleep(50 * time.Millisecond)
		tracker.markScanDone()
	}()

	start := time.Now()
	snapshot, err = tracker.getState(100 * time.Millisecond)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.NotNil(t, snapshot)
	assert.GreaterOrEqual(t, duration, 50*time.Millisecond)
}

func TestRegisterAndGetState(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()
	tracker.markScanDone() // unblock getState

	ctrl := gomock.NewController(t)

	// mock full handler
	fullBackupStats := models.NewBackupStats()
	fullBackupStats.TotalRecords.Store(100)
	fullHandler := NewMockCancelableBackupHandler(ctrl)
	fullHandler.EXPECT().GetStats().Return(fullBackupStats).AnyTimes()
	fullHandler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	// mock incremental handler
	incrBackupStats := models.NewBackupStats()
	incrBackupStats.TotalRecords.Store(50)
	incrHandler := NewMockCancelableBackupHandler(ctrl)
	incrHandler.EXPECT().GetStats().Return(incrBackupStats).AnyTimes()
	incrHandler.EXPECT().GetMetrics().Return(&models.Metrics{}).AnyTimes()

	tracker.register(model.BackupTypeFull, fullHandler)
	tracker.register(model.BackupTypeIncremental, incrHandler)

	snapshot, err := tracker.getState(1 * time.Second)
	require.NoError(t, err)
	assert.NotNil(t, snapshot.full)
	assert.NotNil(t, snapshot.incr)
	assert.Equal(t, uint64(100), snapshot.full.TotalRecords)
	assert.Equal(t, uint64(50), snapshot.incr.TotalRecords)
}

func TestClearBackup(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()
	tracker.markScanDone()

	ctrl := gomock.NewController(t)
	handler := NewMockCancelableBackupHandler(ctrl)
	tracker.register(model.BackupTypeFull, handler)

	tracker.clearBackup(model.BackupTypeFull)

	snapshot, err := tracker.getState(1 * time.Second)
	require.NoError(t, err)
	assert.Nil(t, snapshot.full) // handler should be removed

	tracker.register(model.BackupTypeIncremental, handler)
	tracker.clearBackup(model.BackupTypeIncremental)

	snapshotIncr, err := tracker.getState(1 * time.Second)
	require.NoError(t, err)
	assert.Nil(t, snapshotIncr.incr) // handler should be removed
}

func TestClearFailedBackup(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()
	tracker.markScanDone()

	ctrl := gomock.NewController(t)
	handler := NewMockCancelableBackupHandler(ctrl)
	tracker.register(model.BackupTypeFull, handler)

	// clear a failed backup
	tracker.clearBackup(model.BackupTypeFull)

	snapshot, err := tracker.getState(1 * time.Second)
	require.NoError(t, err)
	assert.Nil(t, snapshot.full)                     // handler should be removed
	assert.Nil(t, snapshot.lastRun.FullBackupTime()) // lastRun should not be updated
}

func TestSetLastRun(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()
	tracker.markScanDone()

	backupTime := model.NewFullBackupTime(time.Now())
	tracker.setLastRun(backupTime)

	snapshot, err := tracker.getState(1 * time.Second)
	require.NoError(t, err)
	assert.Equal(t, backupTime, snapshot.lastRun)
}

func TestScanCancellation(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()

	cancel1Called := false
	cancel1 := func() {
		cancel1Called = true
	}

	cancel2Called := false
	cancel2 := func() {
		cancel2Called = true
	}

	// set the first cancel func
	tracker.setScanCancel(cancel1)

	// set the second, which should trigger the first
	tracker.setScanCancel(cancel2)
	assert.True(t, cancel1Called)
	assert.False(t, cancel2Called)

	// now explicitly cancel the second
	tracker.cancelScan()
	assert.True(t, cancel2Called)

	// check that calling cancelScan again does nothing
	cancel2Called = false
	tracker.cancelScan()
	assert.False(t, cancel2Called)
}

func TestMarkScanDone_Idempotency(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()

	// calling markScanDone multiple times should not panic
	tracker.markScanDone()
	tracker.markScanDone()

	// scanDone should be closed
	select {
	case <-tracker.scanDone:
		// success
	default:
		t.Fatal("scanDone should be closed")
	}
}
