package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
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
	GetRoutineState(routineName string) *model.RoutineState
	// GetRunningState returns statistics for all current backups.
	GetRunningState() map[string]*model.RoutineState
	// Cancel stops all ongoing backups for a specific routine.
	Cancel(routineName string)
	// SynchroniseBackupHistory updates the backup registry with the most recent backup timestamps
	// found in the storage backends. It scans all backup routines in parallel.
	SynchroniseBackupHistory()
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
	handlers       *util.SafeMap[registryKey, CancelableBackupHandler]
	lastSuccessful *util.SafeMap[string, *model.LastBackupRun]

	routineLocks  *util.SafeMap[string, *sync.RWMutex] // Protects individual routines during synchronization
	ctx           context.Context
	config        *model.Config
	backends      *BackendHolderImpl
	routineCancel *util.SafeMap[string, context.CancelFunc]
}

var _ RunningBackupsRegistry = (*RunningBackupsRegistryImpl)(nil)

// NewRunningBackupsRegistry creates a new instance of RunningBackupsRegistryImpl.
func NewRunningBackupsRegistry(
	ctx context.Context,
	backends *BackendHolderImpl,
	config *model.Config,
) *RunningBackupsRegistryImpl {
	return &RunningBackupsRegistryImpl{
		ctx:            ctx,
		config:         config,
		backends:       backends,
		handlers:       util.NewSafeMap[registryKey, CancelableBackupHandler](),
		lastSuccessful: util.NewSafeMap[string, *model.LastBackupRun](),
		routineLocks:   util.NewSafeMap[string, *sync.RWMutex](),
		routineCancel:  util.NewSafeMap[string, context.CancelFunc](),
	}
}

func (r *RunningBackupsRegistryImpl) getRoutineLock(routineName string) *sync.RWMutex {
	return r.routineLocks.LoadOrStore(routineName, &sync.RWMutex{})
}

// SynchroniseBackupHistory updates the backup registry with the most recent backup timestamps
// found in the storage backends. It scans all backup routines in parallel.
func (r *RunningBackupsRegistryImpl) SynchroniseBackupHistory() {
	totalStart := time.Now()

	invalidatedRoutines := r.config.PopInvalidatedRoutines()
	if len(invalidatedRoutines) == 0 {
		return
	}

	slog.Info("Starting backup history synchronization", slog.Int("len", len(invalidatedRoutines)))
	var wg sync.WaitGroup
	for _, routineName := range invalidatedRoutines {
		reader, found := r.backends.GetReader(routineName)
		if !found {
			slog.Warn("Skipping not existing routine", slog.String("routine", routineName))
			continue
		}

		wg.Add(1)
		func(routineName string, routineReader BackupMetadataReader) {
			defer wg.Done()
			// cancel previous scan
			r.routineCancel.Apply(routineName, func(cancel context.CancelFunc) {
				slog.Info("Cancelling previous scan", slog.String("routine", routineName))
				cancel()
			})

			r.scanForRoutine(routineName, routineReader)
		}(routineName, reader)
	}

	wg.Wait()
	slog.Info("Finished backup history synchronization",
		slog.Any("routines", invalidatedRoutines),
		slog.Any("len", len(invalidatedRoutines)),
		slog.Duration("duration", time.Since(totalStart)),
	)
}

func (r *RunningBackupsRegistryImpl) scanForRoutine(routineName string, routineReader BackupMetadataReader) {
	routineLock := r.getRoutineLock(routineName)
	routineLock.Lock()
	defer routineLock.Unlock()

	slog.Info("Start find last backup run", slog.String("routine", routineName))
	ctx, cancelFunc := context.WithCancel(r.ctx)
	r.routineCancel.Store(routineName, cancelFunc)
	defer cancelFunc()

	routineStart := time.Now()
	lastRun, err := findLastRun(ctx, routineReader)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			slog.Info("Backup history scan cancelled", slog.String("routine", routineName))
		case errors.Is(err, errBackupNotFound):
			slog.Info("Backup not found", slog.String("routine", routineName))
		default:
			slog.Error("Failed to read last backup time",
				slog.String("routine", routineName),
				slog.Any("error", err))
		}
		r.lastSuccessful.Remove(routineName)

		return
	}

	slog.Info("Last backup time",
		slog.Duration("duration", time.Since(routineStart)),
		slog.String("routine", routineName),
		slog.String("lastRun", lastRun.String()))

	r.lastSuccessful.Store(routineName, lastRun)
}

// register adds a new backup handler for a specific routine and job type.
func (r *RunningBackupsRegistryImpl) register(routineName string, job jobType, handler CancelableBackupHandler) {
	r.handlers.Store(makeRegistryKey(routineName, job), handler)
}

// unregister removes a backup from the registry and updates the last success timestamp.
// Should be called after successful backup completion.
func (r *RunningBackupsRegistryImpl) unregister(routineName string, job jobType, timestamp time.Time) {
	r.remove(routineName, job)

	routineLock := r.getRoutineLock(routineName)
	routineLock.Lock()
	defer routineLock.Unlock()

	r.setLastTime(routineName, job, timestamp)
}

func (r *RunningBackupsRegistryImpl) setLastTime(routineName string, job jobType, timestamp time.Time) {
	slog.Info("set last backup time",
		slog.String("routine", routineName),
		slog.String("time", timestamp.String()),
		slog.String("job", string(job)),
	)
	updateLastTimestamp := func(lastBackupRun *model.LastBackupRun) {
		if job == jobTypeFull {
			lastBackupRun.SetFullBackupTime(&timestamp)
		} else if job == jobTypeIncremental {
			lastBackupRun.SetIncrementalBackupTime(&timestamp)
		}
	}

	r.lastSuccessful.ApplyOrCreate(
		routineName,
		updateLastTimestamp,
		model.NewLastBackupRun(&timestamp, nil), // it was first backup, always full.
	)
}

// remove deletes a backup from the registry.
// Should be called for failed backups.
func (r *RunningBackupsRegistryImpl) remove(routineName string, job jobType) {
	r.handlers.Remove(makeRegistryKey(routineName, job))
}

// GetRoutineState returns the current backup statistics for a routine.
func (r *RunningBackupsRegistryImpl) GetRoutineState(routineName string) *model.RoutineState {
	fullBackupHandler, _ := r.handlers.Load(makeRegistryKey(routineName, jobTypeFull))
	incrBackupHandler, _ := r.handlers.Load(makeRegistryKey(routineName, jobTypeIncremental))

	routineLock := r.getRoutineLock(routineName)
	routineLock.RLock()
	defer routineLock.RUnlock()

	lastRun, found := r.lastSuccessful.Load(routineName)
	if !found {
		slog.Info("No last backup info available", slog.String("routine", routineName))
		lastRun = &model.LastBackupRun{}
	}

	return &model.RoutineState{
		Full:        currentBackupStatus(fullBackupHandler),
		Incremental: currentBackupStatus(incrBackupHandler),
		LastRunTime: lastRun,
	}
}

// GetRunningState returns statistics for all current backups.
func (r *RunningBackupsRegistryImpl) GetRunningState() map[string]*model.RoutineState {
	var routines []string
	r.handlers.Iterate(func(key registryKey, _ CancelableBackupHandler) {
		if !slices.Contains(routines, key.routineName) { // same routine can be stored twice: as full and incr.
			routines = append(routines, key.routineName)
		}
	})

	stats := make(map[string]*model.RoutineState, len(routines))
	for _, routineName := range routines {
		stats[routineName] = r.GetRoutineState(routineName)
	}

	return stats
}

// Cancel stops all ongoing backups for a specific routine.
func (r *RunningBackupsRegistryImpl) Cancel(routineName string) {
	for _, job := range []jobType{jobTypeFull, jobTypeIncremental} {
		key := makeRegistryKey(routineName, job)
		if handler, found := r.handlers.Load(key); found {
			handler.Cancel()
		}
	}
}

func findLastRun(ctx context.Context, b BackupMetadataReader) (*model.LastBackupRun, error) {
	lastFullBackup, err := b.LastBackupTime(ctx, model.TimeBounds{}, jobTypeFull)
	if err != nil {
		return nil, fmt.Errorf("read last full backup failed: %w", err)
	}
	lastIncrBackup, err := b.LastBackupTime(ctx, model.TimeBounds{
		FromTime: &lastFullBackup,
	}, jobTypeIncremental)

	if err != nil {
		return nil, fmt.Errorf("read last incremental backup failed: %w", err)
	}

	return model.NewLastBackupRun(&lastFullBackup, &lastIncrBackup), nil
}
