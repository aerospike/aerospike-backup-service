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

// TestEndScan_TakesTrackerLock pins the invariant the handover depends on: endScan closes the
// scan channel under the tracker lock, so its check-and-close cannot interleave with the one in
// beginScan. Holding the lock here must block endScan; if it does not, both can observe the same
// open channel and both close it, which panics.
func TestEndScan_TakesTrackerLock(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()

	ch := tracker.beginScan()

	tracker.mu.Lock()

	ended := make(chan struct{})
	go func() {
		defer close(ended)

		tracker.endScan(ch)
	}()

	select {
	case <-ended:
		tracker.mu.Unlock()
		t.Fatal("endScan closed the scan channel without holding the tracker lock; " +
			"a concurrent beginScan closing the same channel would panic")
	case <-time.After(100 * time.Millisecond):
		// Correct: endScan is waiting for the lock.
	}

	tracker.mu.Unlock()

	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("endScan did not complete after the tracker lock was released")
	}

	select {
	case <-ch:
	default:
		t.Fatal("scan channel should be closed once endScan returns")
	}
}

// TestBeginEndScan_ConcurrentCloseIsSerialised covers the handover between a scan that is
// finishing and the next one starting: endScan and beginScan target the same channel, and
// closing it twice panics. The panic would happen on a scan goroutine, where nothing recovers
// it, so it would take the process down rather than fail a single operation.
func TestBeginEndScan_ConcurrentCloseIsSerialised(t *testing.T) {
	t.Parallel()

	const handovers = 2000

	var (
		mu        sync.Mutex
		recovered []any
	)

	recordPanic := func() {
		if r := recover(); r != nil {
			mu.Lock()
			recovered = append(recovered, r)
			mu.Unlock()
		}
	}

	for range handovers {
		tracker := newRoutineTracker()

		// A scan is in progress; its endScan is still pending.
		inFlight := tracker.beginScan()

		var wg sync.WaitGroup

		// The in-flight scan finishes.
		wg.Go(func() {
			defer recordPanic()

			tracker.endScan(inFlight)
		})

		// The next scan starts, which closes the in-flight channel and installs its own.
		wg.Go(func() {
			defer recordPanic()

			tracker.endScan(tracker.beginScan())
		})

		wg.Wait()
	}

	mu.Lock()
	defer mu.Unlock()
	require.Empty(t, recovered, "beginScan and endScan must not both close the same scan channel")
}

// TestBeginEndScan_LateEndScanKeepsCurrentScanOpen checks the handover leaves the tracker
// usable: a stale endScan must not close the channel the newest scan installed, or getState
// would return before that scan has stored its result.
func TestBeginEndScan_LateEndScanKeepsCurrentScanOpen(t *testing.T) {
	t.Parallel()
	tracker := newRoutineTracker()

	stale := tracker.beginScan()
	current := tracker.beginScan() // closes stale, installs current

	tracker.endScan(stale) // late arrival from the superseded scan

	select {
	case <-current:
		t.Fatal("the in-progress scan channel was closed by a superseded scan")
	default:
	}

	_, err := tracker.getState(50 * time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded, "getState must still wait for the current scan")

	tracker.endScan(current)

	snapshot, err := tracker.getState(time.Second)
	require.NoError(t, err)
	require.NotNil(t, snapshot)
}
