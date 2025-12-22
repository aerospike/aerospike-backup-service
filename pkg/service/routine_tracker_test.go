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
	assert.NotNil(t, tracker.initialSyncDone)
	// initialSyncDone should be open
	select {
	case <-tracker.initialSyncDone:
		t.Fatal("initialSyncDone should be open")
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
		tracker.signalSyncDone()
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
	tracker.signalSyncDone() // unblock getState

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

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

	tracker.register(jobTypeFull, fullHandler)
	tracker.register(jobTypeIncremental, incrHandler)

	snapshot, err := tracker.getState(1 * time.Second)
	require.NoError(t, err)
	assert.NotNil(t, snapshot.full)
	assert.NotNil(t, snapshot.incr)
	assert.Equal(t, uint64(100), snapshot.full.TotalRecords)
	assert.Equal(t, uint64(50), snapshot.incr.TotalRecords)
}

func TestRecordSuccessfulBackup(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()
	tracker.signalSyncDone()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	handler := NewMockCancelableBackupHandler(ctrl)
	tracker.register(jobTypeFull, handler)

	// record a successful full backup
	now := time.Now()
	tracker.recordSuccessfulBackup(routineName, jobTypeFull, now)

	snapshot, err := tracker.getState(1 * time.Second)
	require.NoError(t, err)
	assert.Nil(t, snapshot.full) // handler should be removed
	assert.Equal(t, now, *snapshot.lastRun.FullBackupTime())
	assert.Nil(t, snapshot.lastRun.IncrementalBackupTime())

	// record a successful incremental backup
	tracker.register(jobTypeIncremental, handler)
	nowIncr := time.Now()
	tracker.recordSuccessfulBackup(routineName, jobTypeIncremental, nowIncr)

	snapshotIncr, err := tracker.getState(1 * time.Second)
	require.NoError(t, err)
	assert.Nil(t, snapshotIncr.incr) // handler should be removed
	assert.Equal(t, now, *snapshotIncr.lastRun.FullBackupTime())
	assert.Equal(t, nowIncr, *snapshotIncr.lastRun.IncrementalBackupTime())
}

func TestClearFailedBackup(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()
	tracker.signalSyncDone()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	handler := NewMockCancelableBackupHandler(ctrl)
	tracker.register(jobTypeFull, handler)

	// clear a failed backup
	tracker.clearFailedBackup(jobTypeFull)

	snapshot, err := tracker.getState(1 * time.Second)
	require.NoError(t, err)
	assert.Nil(t, snapshot.full)                     // handler should be removed
	assert.Nil(t, snapshot.lastRun.FullBackupTime()) // lastRun should not be updated
}

func TestSetLastRun(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()
	tracker.signalSyncDone()

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

func TestSignalSyncDone_Idempotency(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()

	// calling signalSyncDone multiple times should not panic
	tracker.signalSyncDone()
	tracker.signalSyncDone()

	// channel should be closed
	select {
	case <-tracker.initialSyncDone:
		// success
	default:
		t.Fatal("initialSyncDone should be closed")
	}
}
