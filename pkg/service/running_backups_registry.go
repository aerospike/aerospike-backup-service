package service

import (
	"context"
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
	StartBackupHistorySync(ctx context.Context, backends BackendsHolder)
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

	// ready synchronizes access to backup state data
	// It ensures all last backup timestamps are loaded from storage backends
	// before allowing access through GetRoutineState/GetRunningState.
	ready sync.WaitGroup
}

var _ RunningBackupsRegistry = (*RunningBackupsRegistryImpl)(nil)

// NewRunningBackupsRegistry creates a new instance of RunningBackupsRegistryImpl.
func NewRunningBackupsRegistry(ctx context.Context, backends BackendsHolder) *RunningBackupsRegistryImpl {
	registry := &RunningBackupsRegistryImpl{
		handlers:       util.NewSafeMap[registryKey, CancelableBackupHandler](),
		lastSuccessful: util.NewSafeMap[string, *model.LastBackupRun](),
	}

	registry.StartBackupHistorySync(ctx, backends)
	return registry
}

// StartBackupHistorySync updates the backup registry with the most recent backup timestamps
// found in the storage backends. It scans all backup routines in parallel.
func (r *RunningBackupsRegistryImpl) StartBackupHistorySync(ctx context.Context, backends BackendsHolder) {
	slog.Info("Start backup history sync")
	for routine, reader := range backends.GetAllReaders() {
		r.ready.Add(1)
		go func(routineName string, routineReader BackupMetadataReader) {
			defer r.ready.Done()

			if _, ok := r.lastSuccessful.Load(routineName); ok {
				return // already initialized
			}

			lastRun := routineReader.FindLastRun(ctx)
			slog.Info("Last run", slog.String(routine, lastRun.String()))

			if lastRun.FullBackupTime() != nil {
				r.setLastTime(routineName, jobTypeFull, *lastRun.FullBackupTime())
			}
			if lastRun.IncrementalBackupTime() != nil {
				r.setLastTime(routineName, jobTypeIncremental, *lastRun.IncrementalBackupTime())
			}
		}(routine, reader)
	}
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
	r.ready.Wait() // ensure backups are synced.

	fullBackupHandler, _ := r.handlers.Load(makeRegistryKey(routineName, jobTypeFull))
	incrBackupHandler, _ := r.handlers.Load(makeRegistryKey(routineName, jobTypeIncremental))

	lastRun, found := r.lastSuccessful.Load(routineName)
	if !found {
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
	r.ready.Wait() // ensure backups are synced.

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
