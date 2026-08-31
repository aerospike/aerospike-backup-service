package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/prometheus"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/timeutil"
)

// BackupStateRegistry tracks running backups and the last known backup times for each routine.
type BackupStateRegistry interface {
	// GetRoutineState returns the current backup statistics for a routine.
	GetRoutineState(routine *model.BackupRoutine) model.RoutineState
	// GetRunningState returns statistics for all currently running backups.
	GetRunningState() map[string]model.RoutineState
	// Cancel stops all ongoing backups for a specific routine.
	Cancel(routineName string)
	// SynchroniseBackupHistory updates last backup times from storage for the given routines.
	// It scans the routines in parallel.
	SynchroniseBackupHistory(ctx context.Context, routines []*model.BackupRoutine)

	// BackupStarted stores the handler of a started backup, so it can be tracked and canceled.
	BackupStarted(routineName string, backupType model.BackupType, handler CancelableBackupHandler)
	// BackupSucceeded drops the handler and rescans storage to refresh the last backup time.
	BackupSucceeded(routine *model.BackupRoutine, backupType model.BackupType)
	// BackupFailed drops the handler of a failed backup.
	BackupFailed(routineName string, backupType model.BackupType)
}

// routineProvider supplies the configured backup routines. It is the part of *model.Config
// that callers need, so tests can provide routines without building a whole configuration.
type routineProvider interface {
	Routines() map[string]*model.BackupRoutine
}

var _ routineProvider = (*model.Config)(nil)

type backupStateRegistry struct {
	// trackers holds the state for all known routines
	trackers *collections.SafeMap[string, *routineTracker]

	// history fetches last existing backup
	history HistoryManager

	// config is needed to calculate next run times
	config routineProvider
}

var _ BackupStateRegistry = (*backupStateRegistry)(nil)

const getStateTimeout = 15 * time.Second

// NewBackupStateRegistry returns a BackupStateRegistry.
func NewBackupStateRegistry(
	history HistoryManager,
	config routineProvider,
) BackupStateRegistry {
	return &backupStateRegistry{
		trackers: collections.NewSafeMap[string, *routineTracker](),
		history:  history,
		config:   config,
	}
}

// getTracker atomically retrieves or creates a new tracker for a routine.
func (r *backupStateRegistry) getTracker(routineName string) *routineTracker {
	return r.trackers.LoadOrStore(routineName, newRoutineTracker())
}

// SynchroniseBackupHistory updates the backup registry with the most recent backup timestamps
// found in the storage backends. It scans provided routines in parallel.
func (r *backupStateRegistry) SynchroniseBackupHistory(ctx context.Context, routines []*model.BackupRoutine) {
	if len(routines) == 0 {
		return
	}

	names := make([]string, len(routines))
	for i, t := range routines {
		names[i] = t.Name
	}

	slog.Info("Start backup history synchronization",
		slog.Any("routines", names),
		slog.Int("len", len(routines)),
	)

	duration, err := timeutil.MeasureDuration(func() error {
		return r.scanRoutinesHistory(ctx, routines)
	})

	if err == nil {
		slog.Info("History synchronization completed",
			slog.Any("routines", names),
			slog.Int("len", len(routines)),
			slog.Duration("duration", duration),
		)
		return
	}

	if errors.Is(err, context.Canceled) {
		slog.Info("History synchronization context canceled")
		return
	}

	slog.Error("History synchronization failed", attr.Error(err))
}

func (r *backupStateRegistry) scanRoutinesHistory(ctx context.Context, routines []*model.BackupRoutine) error {
	var (
		errs  error
		errMu sync.Mutex
		wg    sync.WaitGroup
	)

	for _, routine := range routines {
		wg.Go(func() {
			err := r.scanSingleRoutineHistory(ctx, routine)

			errMu.Lock()
			errs = errors.Join(errs, err)
			errMu.Unlock()
		})
	}

	wg.Wait()

	return errs
}

func (r *backupStateRegistry) scanSingleRoutineHistory(ctx context.Context, routine *model.BackupRoutine) error {
	tracker := r.getTracker(routine.Name)

	// Cancel any previous scan that might still be running
	tracker.cancelScan()

	// beginScan unblocks getState callers waiting on the canceled scan
	// and installs a new channel. endScan closes it when this scan finishes,
	// so getState always sees post-scan data.
	scanCh := tracker.beginScan()
	defer tracker.endScan(scanCh)

	ctxWithCancel, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()
	tracker.setScanCancel(cancelFunc)

	lastRun, duration, err := timeutil.MeasureDurationWithResult(func() (*model.BackupTime, error) {
		return r.history.FindLastRun(ctxWithCancel, routine)
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}

		slog.Error("Failed to read last backup time during sync",
			attr.Error(err), attr.Routine(routine.Name),
			slog.Duration("duration", duration),
		)
		tracker.setLastRun(model.NewNoBackupTime())
		return err
	}

	slog.Info("Last existing backup",
		attr.Routine(routine.Name),
		slog.Any("time", lastRun),
		slog.Duration("duration", duration),
	)
	tracker.setLastRun(lastRun)
	prometheus.SetLastBackupTimestamp(routine.Name, lastRun)

	return nil
}

// BackupStarted adds a new backup handler for a specific routine and job type.
func (r *backupStateRegistry) BackupStarted(
	routineName string,
	backupType model.BackupType,
	handler CancelableBackupHandler,
) {
	r.getTracker(routineName).register(backupType, handler)
}

// BackupSucceeded removes a backup from the registry and triggers a storage scan
// to update the last backup timestamp. Storage is the single source of truth for history.
func (r *backupStateRegistry) BackupSucceeded(
	routine *model.BackupRoutine,
	backupType model.BackupType,
) {
	r.getTracker(routine.Name).clearBackup(backupType)
	_ = r.scanSingleRoutineHistory(context.Background(), routine)
}

// BackupFailed deletes a backup from the registry.
func (r *backupStateRegistry) BackupFailed(routineName string, backupType model.BackupType) {
	r.getTracker(routineName).clearBackup(backupType)
}

// GetRoutineState returns the current backup statistics for a routine.
func (r *backupStateRegistry) GetRoutineState(routine *model.BackupRoutine) model.RoutineState {
	tracker := r.getTracker(routine.Name)

	snapshot, err := tracker.getState(getStateTimeout)
	if err != nil {
		slog.Error("Failed to get routine state within timeout", attr.Error(err), attr.Routine(routine.Name))
		// Return a state indicating that history is not available yet
		return model.RoutineState{
			LastRunTime: model.NewNoBackupTime(),
			NextRunTime: model.NewNoBackupTime(),
		}
	}

	nextRunTime, err := nextBackup(routine)
	if err != nil {
		slog.Default().With(attr.Routine(routine.Name)).
			Warn("Failed to calculate next fire time", attr.Error(err))
		nextRunTime = model.NewNoBackupTime()
	}

	return model.RoutineState{
		Full:        snapshot.full,
		Incremental: snapshot.incr,
		LastRunTime: snapshot.lastRun,
		NextRunTime: nextRunTime,
	}
}

// GetRunningState returns statistics for all currently running backups.
func (r *backupStateRegistry) GetRunningState() map[string]model.RoutineState {
	stats := make(map[string]model.RoutineState)

	for _, routine := range r.config.Routines() {
		state := r.GetRoutineState(routine)
		if state.Full != nil || state.Incremental != nil {
			stats[routine.Name] = state
		}
	}

	return stats
}

// Cancel stops all ongoing backups for a specific routine.
func (r *backupStateRegistry) Cancel(routineName string) {
	if tracker, ok := r.trackers.Load(routineName); ok {
		tracker.cancel()
	}
}

func nextBackup(routine *model.BackupRoutine) (*model.BackupTime, error) {
	nextFullBackup, err := timeutil.NextTrigger(routine.IntervalCron, routine.Timezone)
	if err != nil {
		return nil, fmt.Errorf("failed to parse full backup cron: %w", err)
	}

	if routine.IncrIntervalCron == "" {
		return model.NewFullBackupTime(nextFullBackup), nil
	}

	nextIncrementalBackup, err := timeutil.NextTrigger(routine.IncrIntervalCron, routine.Timezone)
	if err != nil {
		return nil, fmt.Errorf("failed to parse incremental backup cron: %w", err)
	}

	return model.NewBackupTime(nextFullBackup, nextIncrementalBackup), nil
}
