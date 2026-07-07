package service

import (
	"context"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// routineTracker holds all state for a single backup routine.
// It is fully thread-safe and manages its own internal locking.
type routineTracker struct {
	// mu protects all fields within this struct
	mu sync.RWMutex

	// --- Live State ---
	handlers map[model.BackupType]CancelableBackupHandler

	// --- History State ---
	lastRun *model.BackupTime

	// --- Scan State ---
	// scanDone is closed when no scan is in progress. getState blocks on it
	// so that callers always see post-scan data.
	// Open (blocking) → a scan is pending or in progress.
	// Closed (non-blocking) → the most recent scan has finished.
	scanDone chan struct{}
	// scanCancel is a function to cancel a currently running history scan
	scanCancel context.CancelFunc
}

// newRoutineTracker creates a new, initialized tracker.
func newRoutineTracker() *routineTracker {
	return &routineTracker{
		scanDone: make(chan struct{}), // open = blocks until first scan completes
		handlers: make(map[model.BackupType]CancelableBackupHandler),
		lastRun:  model.NewNoBackupTime(),
	}
}

// trackerSnapshot holds a point-in-time snapshot of a routine's state.
type trackerSnapshot struct {
	full    *model.RunningJob
	incr    *model.RunningJob
	lastRun *model.BackupTime
}

// getState returns a consistent, point-in-time snapshot of the routine's state.
// It blocks until no storage scan is in progress, so that the returned
// timestamps always reflect completed scan results.
func (t *routineTracker) getState(timeout time.Duration) (*trackerSnapshot, error) {
	deadline := time.After(timeout)

	for {
		t.mu.RLock()
		done := t.scanDone
		t.mu.RUnlock()

		select {
		case <-done:
		case <-deadline:
			return nil, context.DeadlineExceeded
		}

		// If a new scan started while we were waiting (beginScan closes the
		// old channel and installs a new one), loop to wait for the new scan.
		t.mu.RLock()
		if t.scanDone == done {
			snapshot := &trackerSnapshot{
				full:    currentBackupStatus(t.handlers[model.BackupTypeFull]),
				incr:    currentBackupStatus(t.handlers[model.BackupTypeIncremental]),
				lastRun: t.lastRun,
			}
			t.mu.RUnlock()

			return snapshot, nil
		}
		t.mu.RUnlock()
	}
}

// register adds a new running backup handler.
func (t *routineTracker) register(backupType model.BackupType, handler CancelableBackupHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Set handler
	t.handlers[backupType] = handler
}

// clearBackup removes a backup handler from the tracker.
func (t *routineTracker) clearBackup(backupType model.BackupType) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.handlers, backupType)
}

// setLastRun updates the history state from a storage scan result.
// It replaces the entire lastRun value, making storage the single source of truth.
func (t *routineTracker) setLastRun(lastRun *model.BackupTime) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lastRun = lastRun
	t.scanCancel = nil
}

// cancel stops all ongoing backups for this routine.
func (t *routineTracker) cancel() {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, handler := range t.handlers {
		handler.Cancel()
	}
}

// setScanCancel stores the cancel function for the current history scan.
// It cancels any previously running scan.
func (t *routineTracker) setScanCancel(cancel context.CancelFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.scanCancel != nil {
		t.scanCancel()
	}
	t.scanCancel = cancel
}

// cancelScan actively cancels a running history scan.
func (t *routineTracker) cancelScan() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.scanCancel != nil {
		t.scanCancel()
		t.scanCancel = nil
	}
}

// beginScan prepares the tracker for a new storage scan.
// It unblocks any getState callers waiting on a previous (now-canceled) scan
// and installs a fresh open channel for the new scan.
// Returns the channel that the caller must pass to endScan when done.
func (t *routineTracker) beginScan() chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()

	closeChan(t.scanDone)

	ch := make(chan struct{})
	t.scanDone = ch

	return ch
}

// endScan signals that a specific scan has completed.
// Safe to call if the channel was already closed (e.g. by a subsequent beginScan).
func (t *routineTracker) endScan(ch chan struct{}) {
	closeChan(ch)
}

// markScanDone closes the current scanDone channel without starting a new scan.
// Used in tests to skip the scan wait.
func (t *routineTracker) markScanDone() {
	t.mu.Lock()
	defer t.mu.Unlock()

	closeChan(t.scanDone)
}

func closeChan(ch chan struct{}) {
	select {
	case <-ch:
	default:
		close(ch)
	}
}
