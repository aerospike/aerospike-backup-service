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
	fullHandler CancelableBackupHandler
	incrHandler CancelableBackupHandler

	// --- History State ---
	lastRun *model.BackupTime

	// --- Sync State ---
	// initialSyncDone is closed when the *first* history scan is completed.
	// This ensures getState calls block until history is populated.
	initialSyncDone chan struct{}
}

// newRoutineTracker creates a new, initialized tracker.
func newRoutineTracker() *routineTracker {
	return &routineTracker{
		initialSyncDone: make(chan struct{}),
		lastRun:         model.NewNoBackupTime(), // Start with non-nil empty state
	}
}

// getState returns a consistent, point-in-time snapshot of the routine's state.
// It will block until the initial history synchronization is complete.
func (t *routineTracker) getState(timeout time.Duration) (
	full *model.RunningJob,
	incr *model.RunningJob,
	lastRun *model.BackupTime,
	err error,
) {
	select {
	case <-t.initialSyncDone:
		// Sync is done, proceed
	case <-time.After(timeout):
		return nil, nil, nil, context.DeadlineExceeded
	}

	// Now that sync is done, get a consistent snapshot of the state
	t.mu.RLock()
	defer t.mu.RUnlock()

	return currentBackupStatus(t.fullHandler),
		currentBackupStatus(t.incrHandler),
		t.lastRun,
		nil
}

// register adds a new running backup handler.
func (t *routineTracker) register(job jobType, handler CancelableBackupHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if job == jobTypeFull {
		t.fullHandler = handler
	} else {
		t.incrHandler = handler
	}
}

// recordSuccessfulBackup removes a successful backup and updates its last run time.
func (t *routineTracker) recordSuccessfulBackup(routineName string, job jobType, timestamp time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	logger := slog.Default().With(attr.Routine(routineName))
	logger.Info("set last backup time",
		slog.String("time", timestamp.String()),
		slog.String("job", string(job)),
	)

	// Ensure lastRun is not nil
	if t.lastRun == nil {
		t.lastRun = model.NewNoBackupTime()
	}

	switch job {
	case jobTypeFull:
		t.lastRun.SetFullBackupTime(&timestamp)
	case jobTypeIncremental:
		t.lastRun.SetIncrementalBackupTime(&timestamp)
	}

	// set last successful backup time for just finished backup
	lastBackupTimestamp.WithLabelValues(routineName).Set(float64(timestamp.Unix()))

	// Remove the handler
	if job == jobTypeFull {
		t.fullHandler = nil
	} else {
		t.incrHandler = nil
	}
}

// clearFailedBackup removes a failed backup handler without updating history.
func (t *routineTracker) clearFailedBackup(job jobType) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if job == jobTypeFull {
		t.fullHandler = nil
	} else {
		t.incrHandler = nil
	}
}

// setLastRun updates the history state.
// This is called by the HistoryManager after a scan.
func (t *routineTracker) setLastRun(lastRun *model.BackupTime) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.signalSyncDone()

	t.lastRun = lastRun
}

// cancel stops all ongoing backups for this routine.
func (t *routineTracker) cancel() {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.fullHandler != nil {
		t.fullHandler.Cancel()
	}
	if t.incrHandler != nil {
		t.incrHandler.Cancel()
	}
}

// signalSyncDone safely closes the initialSyncDone channel.
func (t *routineTracker) signalSyncDone() {
	// This select prevents a panic if the channel is already closed
	select {
	case <-t.initialSyncDone:
		// Channel already closed.
	default:
		// Channel is open, close it.
		close(t.initialSyncDone)
	}
}
