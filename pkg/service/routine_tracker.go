package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
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

	// --- Sync State ---
	// initialSyncDone is closed when the *first* history scan is completed.
	// This ensures getState calls block until history is populated.
	initialSyncDone chan struct{}
	syncOnce        sync.Once

	// --- Scan State ---
	// scanCancel is a function to cancel a currently running history scan
	scanCancel context.CancelFunc
}

// newRoutineTracker creates a new, initialized tracker.
func newRoutineTracker() *routineTracker {
	return &routineTracker{
		initialSyncDone: make(chan struct{}),
		handlers:        make(map[model.BackupType]CancelableBackupHandler),
		lastRun:         model.NewNoBackupTime(), // Start with non-nil empty state
	}
}

// trackerSnapshot holds a point-in-time snapshot of a routine's state.
type trackerSnapshot struct {
	full    *model.RunningJob
	incr    *model.RunningJob
	lastRun *model.BackupTime
}

// getState returns a consistent, point-in-time snapshot of the routine's state.
// It will block until the initial history synchronization is complete.
func (t *routineTracker) getState(timeout time.Duration) (*trackerSnapshot, error) {
	select {
	case <-t.initialSyncDone:
		// Sync is done, proceed
	case <-time.After(timeout):
		return nil, context.DeadlineExceeded
	}

	// Now that sync is done, get a consistent snapshot of the state
	t.mu.RLock()
	defer t.mu.RUnlock()

	return &trackerSnapshot{
		full:    currentBackupStatus(t.handlers[model.BackupTypeFull]),
		incr:    currentBackupStatus(t.handlers[model.BackupTypeIncremental]),
		lastRun: t.lastRun,
	}, nil
}

// register adds a new running backup handler.
func (t *routineTracker) register(backupType model.BackupType, handler CancelableBackupHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Set handler
	t.handlers[backupType] = handler
}

// recordSuccessfulBackup removes a successful backup and updates its last run time.
func (t *routineTracker) recordSuccessfulBackup(routineName string, backupType model.BackupType, timestamp time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	logger := slog.Default().With(attr.Routine(routineName))
	logger.Info("Set last backup time",
		slog.String("time", timestamp.String()),
		slog.String("job", string(backupType)),
	)

	switch backupType {
	case model.BackupTypeFull:
		t.lastRun.SetFullBackupTime(timestamp)
	case model.BackupTypeIncremental:
		t.lastRun.SetIncrementalBackupTime(timestamp)
	}

	// Remove the handler
	delete(t.handlers, backupType)
}

// clearFailedBackup removes a failed backup handler without updating history.
func (t *routineTracker) clearFailedBackup(backupType model.BackupType) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Remove the handler
	delete(t.handlers, backupType)
}

// setLastRun updates the history state.
// This is called by the HistoryManagerImpl after a scan.
// It merges rather than replaces, so that a concurrent recordSuccessfulBackup
// (fired as a goroutine from backup_completion) cannot have its more recent
// timestamp silently overwritten by a stale storage-scan result.
func (t *routineTracker) setLastRun(lastRun *model.BackupTime) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if full := lastRun.FullBackupTime(); full != nil {
		if cur := t.lastRun.FullBackupTime(); cur == nil || full.After(*cur) {
			t.lastRun.SetFullBackupTime(*full)
		}
	}

	if incr := lastRun.IncrementalBackupTime(); incr != nil {
		if cur := t.lastRun.IncrementalBackupTime(); cur == nil || incr.After(*cur) {
			t.lastRun.SetIncrementalBackupTime(*incr)
		}
	}

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

// signalSyncDone safely closes the initialSyncDone channel.
func (t *routineTracker) signalSyncDone() {
	t.syncOnce.Do(func() {
		close(t.initialSyncDone)
	})
}
