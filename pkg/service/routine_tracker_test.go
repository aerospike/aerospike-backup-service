package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// markScanDone completes a history scan with the production beginScan/endScan
// handshake so getState does not block. Tests that do not exercise storage
// scans call this instead of SynchroniseBackupHistory.
func (t *routineTracker) markScanDone() {
	t.endScan(t.beginScan())
}

// newScanBackupHandler returns a mock handler of a scan backup that has started and estimated
// totalRecords, none of them read yet. Tests add the expectations they exercise.
func newScanBackupHandler(ctrl *gomock.Controller, totalRecords uint64) *MockRoutineBackupHandler {
	h := NewMockRoutineBackupHandler(ctrl)
	h.EXPECT().Status().Return(&model.BackupRun{
		StartTime: time.Now(),
		Scan:      &model.ScanProgress{TotalRecords: totalRecords, Metrics: &models.Metrics{}},
	}).AnyTimes()

	return h
}

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
	tracker.markScanDone()

	ctrl := gomock.NewController(t)

	// mock full handler
	fullHandler := newScanBackupHandler(ctrl, 100)

	// mock incremental handler
	incrHandler := newScanBackupHandler(ctrl, 50)

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
	handler := NewMockRoutineBackupHandler(ctrl)
	handler.EXPECT().Status().Return(nil).AnyTimes() // finished: nothing runs
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
	handler := NewMockRoutineBackupHandler(ctrl)
	handler.EXPECT().Status().Return(nil).AnyTimes() // finished: nothing runs
	tracker.register(model.BackupTypeFull, handler)

	// clear a failed backup
	tracker.clearBackup(model.BackupTypeFull)

	snapshot, err := tracker.getState(1 * time.Second)
	require.NoError(t, err)
	assert.Nil(t, snapshot.full)                     // handler should be removed
	assert.Nil(t, snapshot.lastRun.FullBackupTime()) // lastRun should not be updated
}

func TestClearBackup_KeepsServerJobs(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()
	tracker.markScanDone()

	ctrl := gomock.NewController(t)
	jobs := []model.ServerBackupJob{{JobID: "job", Namespace: "test", State: "FAILED", ErrorReason: "s3 PUT 403"}}
	finished := NewMockRoutineBackupHandler(ctrl)
	finished.EXPECT().Status().Return(&model.BackupRun{Server: &model.ServerProgress{Jobs: jobs}})
	tracker.register(model.BackupTypeFull, finished)
	tracker.clearBackup(model.BackupTypeFull)

	// The next full backup is running: the last finished one stays reported.
	tracker.register(model.BackupTypeFull, newScanBackupHandler(ctrl, 100))

	snapshot, err := tracker.getState(1 * time.Second)
	require.NoError(t, err)
	assert.Equal(t, jobs, snapshot.lastFullJobs)
	assert.Empty(t, snapshot.lastIncrJobs)
}

// TestClearBackup_NotStarted clears a backup that failed before a handler was registered.
func TestClearBackup_NotStarted(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()
	tracker.markScanDone()

	tracker.clearBackup(model.BackupTypeFull)

	snapshot, err := tracker.getState(1 * time.Second)
	require.NoError(t, err)
	assert.Nil(t, snapshot.full)
	assert.Empty(t, snapshot.lastFullJobs)
}

func TestClearBackup_ScanBackupHasNoServerJobs(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()
	tracker.markScanDone()

	tracker.register(model.BackupTypeFull, newScanBackupHandler(gomock.NewController(t), 100))
	tracker.clearBackup(model.BackupTypeFull)

	snapshot, err := tracker.getState(1 * time.Second)
	require.NoError(t, err)
	assert.Empty(t, snapshot.lastFullJobs)
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

func TestSetLastRun_KeepsScanCancel(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()

	cancelCalled := false
	tracker.setScanCancel(func() { cancelCalled = true })

	// An older scan storing its result must not take the newer scan's handle with it.
	tracker.setLastRun(model.NewNoBackupTime())
	tracker.cancelScan()
	assert.True(t, cancelCalled)
}

func TestFinishScan_Idempotency(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()

	tracker.markScanDone()
	tracker.markScanDone()

	select {
	case <-tracker.scanDone:
	default:
		t.Fatal("scanDone should be closed")
	}
}

// A double close panics inside the goroutine, which crashes the test binary.
func TestBeginEndScan_ConcurrentNoDoubleClose(t *testing.T) {
	for range 2000 {
		tracker := newRoutineTracker()

		// Scan 1 has begun; its endScan is still pending.
		ch1 := tracker.beginScan()

		var wg sync.WaitGroup

		// Scan 1 finishing
		wg.Go(func() {
			tracker.endScan(ch1)
		})

		// Scan 2 starting: beginScan closes tracker.scanDone, which is still ch1.
		wg.Go(func() {
			tracker.endScan(tracker.beginScan())
		})

		wg.Wait()
	}
}
