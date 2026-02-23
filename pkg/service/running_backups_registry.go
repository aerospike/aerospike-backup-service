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
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/timeutil"
)

// RunningBackupsRegistry defines the interface for managing running backups and their statuses.
type RunningBackupsRegistry interface {
	// register adds a new backup handler for a specific routine and job type.
	register(routineName string, jt jobType, handler CancelableBackupHandler)
	// clearFailedBackup deletes a backup from the registry.
	// Should be called for failed backups.
	clearFailedBackup(routineName string, jt jobType)
	// recordSuccessfulBackup removes a backup from the registry and updates the last success timestamp.
	// Should be called after successful backup completion.
	recordSuccessfulBackup(routineName string, jt jobType, timestamp time.Time)

	// GetRoutineState returns the current backup statistics for a routine.
	GetRoutineState(routine *model.BackupRoutine) *model.RoutineState
	// GetRunningState returns statistics for all current backups.
	GetRunningState() map[string]*model.RoutineState
	// Cancel stops all ongoing backups for a specific routine.
	Cancel(routineName string)
	// SynchroniseBackupHistory updates the backup registry with the most recent backup timestamps
	// found in the storage backends. It scans all backup routines in parallel.
	SynchroniseBackupHistory(ctx context.Context)
}

type routineProvider interface {
	PopInvalidatedRoutines() []*model.BackupRoutine
	Routines() map[string]*model.BackupRoutine
}

// RunningBackupsRegistryImpl implements the RunningBackupsRegistry interface.
// It acts as a coordinator, managing a map of per-routine trackers.
type RunningBackupsRegistryImpl struct {
	// trackers holds the state for all known routines
	trackers *collections.SafeMap[string, *routineTracker]

	// history fetches last existing backup
	history HistoryManager

	// config is needed to calculate next run times
	config routineProvider
}

var _ RunningBackupsRegistry = (*RunningBackupsRegistryImpl)(nil)

const getStateTimeout = 5 * time.Second

// NewRunningBackupsRegistry creates a new instance of RunningBackupsRegistryImpl.
func NewRunningBackupsRegistry(
	history HistoryManager,
	config routineProvider,
) *RunningBackupsRegistryImpl {
	return &RunningBackupsRegistryImpl{
		trackers: collections.NewSafeMap[string, *routineTracker](),
		history:  history,
		config:   config,
	}
}

// getTracker atomically retrieves or creates a new tracker for a routine.
func (r *RunningBackupsRegistryImpl) getTracker(routineName string) *routineTracker {
	return r.trackers.LoadOrStore(routineName, newRoutineTracker())
}

// SynchroniseBackupHistory updates the backup registry with the most recent backup timestamps
// found in the storage backends. It scans all backup routines in parallel.
func (r *RunningBackupsRegistryImpl) SynchroniseBackupHistory(ctx context.Context) {
	invalidatedRoutines := r.config.PopInvalidatedRoutines()
	if len(invalidatedRoutines) == 0 {
		return
	}

	names := make([]string, len(invalidatedRoutines))
	for i, t := range invalidatedRoutines {
		names[i] = t.Name
	}

	slog.Info("Start backup history synchronization",
		slog.Any("routines", names),
		slog.Int("len", len(invalidatedRoutines)),
	)

	duration, err := timeutil.MeasureDuration(func() error {
		return r.scanRoutinesHistory(ctx, invalidatedRoutines)
	})

	if err == nil {
		slog.Info("History synchronization completed",
			slog.Any("routines", names),
			slog.Int("len", len(invalidatedRoutines)),
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

func (r *RunningBackupsRegistryImpl) scanRoutinesHistory(ctx context.Context, routines []*model.BackupRoutine) error {
	var (
		errs  error
		errMu sync.Mutex
		wg    sync.WaitGroup
	)

	for _, routine := range routines {
		wg.Add(1)
		go func(routine *model.BackupRoutine) {
			defer wg.Done()
			err := r.scanSingleRoutineHistory(ctx, routine)

			errMu.Lock()
			errs = errors.Join(errs, err)
			errMu.Unlock()
		}(routine)
	}

	wg.Wait()

	return errs
}

func (r *RunningBackupsRegistryImpl) scanSingleRoutineHistory(ctx context.Context, routine *model.BackupRoutine) error {
	tracker := r.getTracker(routine.Name)

	// Cancel any previous scan that might still be running
	tracker.cancelScan()

	// Always signal that the (first) sync is done, even on failure.
	defer tracker.signalSyncDone()

	ctxWithCancel, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()
	tracker.setScanCancel(cancelFunc)

	lastRun, err := r.history.FindLastRun(ctxWithCancel, routine)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Error("Failed to read last backup time during sync", attr.Error(err), attr.Routine(routine.Name))
		}
		tracker.setLastRun(model.NewNoBackupTime())
		return err
	}

	// On success, update the tracker's history
	slog.Info("Last existing backup", attr.Routine(routine.Name), slog.Any("time", lastRun))
	tracker.setLastRun(lastRun)
	if lastRun.LatestRun() != nil {
		lastBackupTimestamp.WithLabelValues(routine.Name).Set(float64(lastRun.LatestRun().Unix()))
	}

	return nil
}

// register adds a new backup handler for a specific routine and job type.
func (r *RunningBackupsRegistryImpl) register(routineName string, job jobType, handler CancelableBackupHandler) {
	r.getTracker(routineName).register(job, handler)
}

// recordSuccessfulBackup removes a backup from the registry and updates the last success timestamp.
func (r *RunningBackupsRegistryImpl) recordSuccessfulBackup(routineName string, job jobType, timestamp time.Time) {
	r.getTracker(routineName).recordSuccessfulBackup(routineName, job, timestamp)
}

// clearFailedBackup deletes a backup from the registry.
func (r *RunningBackupsRegistryImpl) clearFailedBackup(routineName string, job jobType) {
	r.getTracker(routineName).clearFailedBackup(job)
}

// GetRoutineState returns the current backup statistics for a routine.
func (r *RunningBackupsRegistryImpl) GetRoutineState(routine *model.BackupRoutine) *model.RoutineState {
	tracker := r.getTracker(routine.Name)

	snapshot, err := tracker.getState(getStateTimeout)
	if err != nil {
		slog.Error("Failed to get routine state within timeout", attr.Error(err), attr.Routine(routine.Name))
		// Return a state indicating that history is not available yet
		return &model.RoutineState{
			LastRunTime: model.NewNoBackupTime(),
			NextRunTime: model.NewNoBackupTime(),
		}
	}

	nextRunTime, err := nextBackup(routine)
	if err != nil {
		slog.Default().With(attr.Routine(routine.Name)).
			Warn("Could not calculate next fire time", attr.Error(err))
		nextRunTime = model.NewNoBackupTime()
	}

	return &model.RoutineState{
		Full:        snapshot.full,
		Incremental: snapshot.incr,
		LastRunTime: snapshot.lastRun,
		NextRunTime: nextRunTime,
	}
}

// GetRunningState returns statistics for all current backups.
func (r *RunningBackupsRegistryImpl) GetRunningState() map[string]*model.RoutineState {
	stats := make(map[string]*model.RoutineState)

	for _, routine := range r.config.Routines() {
		state := r.GetRoutineState(routine)
		if state.Full != nil || state.Incremental != nil {
			stats[routine.Name] = state
		}
	}

	return stats
}

// Cancel stops all ongoing backups for a specific routine.
func (r *RunningBackupsRegistryImpl) Cancel(routineName string) {
	if tracker, ok := r.trackers.Load(routineName); ok {
		tracker.cancel()
	}
}

func nextBackup(routine *model.BackupRoutine) (*model.BackupTime, error) {
	nextFullBackup, err := timeutil.NextTrigger(routine.IntervalCron)
	if err != nil {
		return nil, fmt.Errorf("failed to parse full backup cron: %w", err)
	}

	if routine.IncrIntervalCron == "" {
		return model.NewFullBackupTime(nextFullBackup), nil
	}

	nextIncrementalBackup, err := timeutil.NextTrigger(routine.IncrIntervalCron)
	if err != nil {
		return nil, fmt.Errorf("failed to parse incremental backup cron: %w", err)
	}

	return model.NewBackupTime(nextFullBackup, nextIncrementalBackup), nil
}
