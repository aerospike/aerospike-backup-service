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
	"github.com/reugn/go-quartz/quartz"
)

// RunningBackupsRegistry defines the interface for managing running backups and their statuses.
type RunningBackupsRegistry interface {
	// register adds a new backup handler for a specific routine and job type.
	register(routineName string, jt jobType, handler CancelableBackupHandler)
	// remove deletes a backup from the registry.
	// Should be called for failed backups.
	remove(routineName string, jt jobType)
	// unregister removes a backup from the registry and updates the last success timestamp.
	// Should be called after successful backup completion.
	unregister(routineName string, jt jobType, timestamp time.Time)

	// GetRoutineState returns the current backup statistics for a routine.
	GetRoutineState(routine *model.BackupRoutine) *model.RoutineState
	// GetRunningState returns statistics for all current backups.
	GetRunningState() map[string]*model.RoutineState
	// Cancel stops all ongoing backups for a specific routine.
	Cancel(routineName string)
	// SynchroniseBackupHistory updates the backup registry with the most recent backup timestamps
	// found in the storage backends. It scans all backup routines in parallel.
	SynchroniseBackupHistory()
}

type routineProvider interface {
	PopInvalidatedRoutines() []*model.BackupRoutine
	Routines() map[string]*model.BackupRoutine
}

type registryKey struct {
	routineName string
	job         jobType
}

func makeRegistryKey(routineName string, job jobType) registryKey {
	return registryKey{
		routineName: routineName,
		job:         job,
	}
}

// RunningBackupsRegistryImpl implements the RunningBackupsRegistry interface.
type RunningBackupsRegistryImpl struct {
	handlers       *collections.SafeMap[registryKey, CancelableBackupHandler]
	lastSuccessful *collections.SafeMap[string, *model.BackupTime]

	routineLocks  collections.LockMap // Protects individual routines during synchronization
	ctx           context.Context
	config        routineProvider
	backupReader  BackupReader
	routineCancel *collections.SafeMap[string, context.CancelFunc]
}

var _ RunningBackupsRegistry = (*RunningBackupsRegistryImpl)(nil)

// NewRunningBackupsRegistry creates a new instance of RunningBackupsRegistryImpl.
func NewRunningBackupsRegistry(
	ctx context.Context,
	backupReader BackupReader,
	config routineProvider,
) *RunningBackupsRegistryImpl {
	return &RunningBackupsRegistryImpl{
		ctx:            ctx,
		config:         config,
		backupReader:   backupReader,
		handlers:       collections.NewSafeMap[registryKey, CancelableBackupHandler](),
		lastSuccessful: collections.NewSafeMap[string, *model.BackupTime](),
		routineCancel:  collections.NewSafeMap[string, context.CancelFunc](),
	}
}

// SynchroniseBackupHistory updates the backup registry with the most recent backup timestamps
// found in the storage backends. It scans all backup routines in parallel.
func (r *RunningBackupsRegistryImpl) SynchroniseBackupHistory() {
	totalStart := time.Now()

	invalidatedRoutines := r.config.PopInvalidatedRoutines()
	if len(invalidatedRoutines) == 0 {
		return
	}

	slog.Info("Start backup history synchronization",
		slog.Any("routines", invalidatedRoutines),
		slog.Int("len", len(invalidatedRoutines)),
	)

	var wg sync.WaitGroup
	for _, routine := range invalidatedRoutines {
		wg.Add(1)
		go func(routine *model.BackupRoutine) {
			defer wg.Done()
			// cancel previous scan
			r.routineCancel.Apply(routine.Name, func(cancel context.CancelFunc) {
				cancel()
			})

			r.scanForRoutine(routine)
		}(routine)
	}

	wg.Wait()
	slog.Info("History synchronization completed",
		slog.Any("routines", invalidatedRoutines),
		slog.Any("len", len(invalidatedRoutines)),
		slog.Duration("duration", time.Since(totalStart)),
	)
}

func (r *RunningBackupsRegistryImpl) scanForRoutine(routine *model.BackupRoutine) {
	routineLock := r.routineLocks.Get(routine.Name)
	routineLock.Lock()
	defer routineLock.Unlock()

	ctx, cancelFunc := context.WithCancel(r.ctx)
	r.routineCancel.Store(routine.Name, cancelFunc)
	defer cancelFunc()

	logger := slog.Default().With(attr.Routine(routine.Name))
	routineStart := time.Now()
	lastRun, err := r.findLastRun(ctx, routine)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			logger.Info("Backup history scan cancelled")
		default:
			logger.Error("Failed to read last backup time", attr.Error(err))
		}
		r.lastSuccessful.Remove(routine.Name)

		return
	}

	logger.Info("Last backup time scan completed",
		slog.Duration("duration", time.Since(routineStart)),
		slog.String("lastRun", lastRun.String()))

	// set last successful backup time for backups done before ABS started
	if lastRun.LatestRun() != nil {
		lastBackupTimestamp.WithLabelValues(routine.Name).Set(float64(lastRun.LatestRun().Unix()))
	}
	r.lastSuccessful.Store(routine.Name, lastRun)
}

// register adds a new backup handler for a specific routine and job type.
func (r *RunningBackupsRegistryImpl) register(routineName string, job jobType, handler CancelableBackupHandler) {
	r.handlers.Store(makeRegistryKey(routineName, job), handler)
}

// unregister removes a backup from the registry and updates the last success timestamp.
// Should be called after successful backup completion.
func (r *RunningBackupsRegistryImpl) unregister(routineName string, job jobType, timestamp time.Time) {
	r.setLastTime(routineName, job, timestamp)
	r.remove(routineName, job)
}

func (r *RunningBackupsRegistryImpl) setLastTime(routineName string, job jobType, timestamp time.Time) {
	routineLock := r.routineLocks.Get(routineName)
	routineLock.Lock()
	defer routineLock.Unlock()

	logger := slog.Default().With(attr.Routine(routineName))
	logger.Info("set last backup time",
		slog.String("time", timestamp.String()),
		slog.String("job", string(job)),
	)

	updateLastTimestamp := func(lastBackupRun *model.BackupTime) {
		switch job {
		case jobTypeFull:
			lastBackupRun.SetFullBackupTime(&timestamp)
		case jobTypeIncremental:
			lastBackupRun.SetIncrementalBackupTime(&timestamp)
		}
	}

	// set last successful backup time for just finished backup
	lastBackupTimestamp.WithLabelValues(routineName).Set(float64(timestamp.Unix()))
	r.lastSuccessful.ApplyOrCreate(
		routineName,
		updateLastTimestamp,
		model.NewFullBackupTime(timestamp), // it was first backup, always full.
	)
}

// remove deletes a backup from the registry.
// Should be called for failed backups.
func (r *RunningBackupsRegistryImpl) remove(routineName string, job jobType) {
	r.handlers.Remove(makeRegistryKey(routineName, job))
}

// GetRoutineState returns the current backup statistics for a routine.
func (r *RunningBackupsRegistryImpl) GetRoutineState(routine *model.BackupRoutine) *model.RoutineState {
	fullBackupHandler, _ := r.handlers.Load(makeRegistryKey(routine.Name, jobTypeFull))
	incrBackupHandler, _ := r.handlers.Load(makeRegistryKey(routine.Name, jobTypeIncremental))

	routineLock := r.routineLocks.Get(routine.Name)
	routineLock.RLock()
	defer routineLock.RUnlock()

	logger := slog.Default().With(attr.Routine(routine.Name))
	lastRun, found := r.lastSuccessful.Load(routine.Name)
	if !found {
		logger.Info("No last backup info available")
		lastRun = model.NewNoBackupTime()
	}

	nextRunTime, err := nextBackup(routine)
	if err != nil {
		logger.Warn("Could not calculate next fire time", attr.Error(err))
		nextRunTime = model.NewNoBackupTime()
	}

	return &model.RoutineState{
		Full:        currentBackupStatus(fullBackupHandler),
		Incremental: currentBackupStatus(incrBackupHandler),
		LastRunTime: lastRun,
		NextRunTime: nextRunTime,
	}
}

func nextBackup(routine *model.BackupRoutine) (*model.BackupTime, error) {
	nextFullBackup, err := nextTrigger(routine.IntervalCron)
	if err != nil {
		return nil, fmt.Errorf("failed to parse full backup cron: %w", err)
	}

	if routine.IncrIntervalCron == "" {
		return model.NewFullBackupTime(nextFullBackup), nil
	}

	nextIncrementalBackup, err := nextTrigger(routine.IncrIntervalCron)
	if err != nil {
		return nil, fmt.Errorf("failed to parse incremental backup cron: %w", err)
	}

	return model.NewBackupTime(nextFullBackup, nextIncrementalBackup), nil
}

func nextTrigger(cron string) (time.Time, error) {
	trigger, err := quartz.NewCronTrigger(cron)
	if err != nil {
		return time.Time{}, err
	}

	fireTime, err := trigger.NextFireTime(time.Now().UnixNano())
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(0, fireTime), nil
}

// GetRunningState returns statistics for all current backups.
func (r *RunningBackupsRegistryImpl) GetRunningState() map[string]*model.RoutineState {
	routines := r.config.Routines()
	stats := make(map[string]*model.RoutineState, len(routines))

	for routineName, routine := range routines {
		stats[routineName] = r.GetRoutineState(routine)
	}

	return stats
}

// Cancel stops all ongoing backups for a specific routine.
func (r *RunningBackupsRegistryImpl) Cancel(routineName string) {
	for _, job := range []jobType{jobTypeFull, jobTypeIncremental} {
		key := makeRegistryKey(routineName, job)
		r.handlers.Apply(key, func(handler CancelableBackupHandler) {
			handler.Cancel()
		})
	}
}

func (r *RunningBackupsRegistryImpl) findLastRun(
	ctx context.Context,
	routine *model.BackupRoutine,
) (*model.BackupTime, error) {
	lastFullBackup, err := r.backupReader.GetBackups(ctx, NewFullBackupFilter(routine).Last())
	if err != nil {
		return nil, fmt.Errorf("read last full backup failed: %w", err)
	}

	if len(lastFullBackup) == 0 {
		return model.NewNoBackupTime(), nil
	}
	lastFullTime := lastFullBackup[0].Created

	lastIncrBackup, err := r.backupReader.GetBackups(ctx,
		NewIncrementalBackupFilter(routine).WithFromTime(lastFullTime).Last())
	if err != nil {
		return nil, fmt.Errorf("read last incremental backup failed: %w", err)
	}

	if len(lastIncrBackup) > 0 {
		return model.NewBackupTime(lastFullTime, lastIncrBackup[0].Created), nil
	}

	return model.NewFullBackupTime(lastFullTime), nil
}
