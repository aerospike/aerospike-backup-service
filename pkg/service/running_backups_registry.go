package service

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
)

// RunningBackupsRegistry defines the interface for managing running backups and their statuses.
type RunningBackupsRegistry interface {
	// register add a new backup handler for a specific routine and job type.
	register(string, jobType, CancelableBackupHandler)
	// finishWithError remove backup from the registry.
	remove(routineName string, job jobType)
	// FinishFull remove backup from registry and update last success timestamp.
	FinishFull(routineName string, time time.Time)
	// FinishIncremental remove incremental backup from registry and update last success timestamp.
	FinishIncremental(routineName string, time time.Time)
	// CurrentStat get the current backup statistics for a routine.
	CurrentStat(string) *model.CurrentBackups
	// GetAllCurrentStats all current backups statistics.
	GetAllCurrentStats() map[string]*model.CurrentBackups
	// Cancel all ongoing backups for a specific routine.
	Cancel(string)
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
}

var _ RunningBackupsRegistry = (*RunningBackupsRegistryImpl)(nil)

// NewRunningBackupsRegistry creates a new instance of RunningBackupsRegistryImpl.
func NewRunningBackupsRegistry() *RunningBackupsRegistryImpl {
	return &RunningBackupsRegistryImpl{
		handlers:       util.NewSafeMap[registryKey, CancelableBackupHandler](),
		lastSuccessful: util.NewSafeMap[string, *model.LastBackupRun](),
	}
}

// SyncBackupHistoryFromStorage updates the backup registry with the most recent backup timestamps
// found in the storage backends. It scans all backup routines in parallel.
func SyncBackupHistoryFromStorage(
	ctx context.Context, registry RunningBackupsRegistry, backends BackendsHolder,
) {
	var wg sync.WaitGroup

	// Launch a goroutine for each backup routine, because routineReader.FindLastRun(ctx) is network call and can be long.
	for routine, reader := range backends.GetAllReaders() {
		wg.Add(1)
		go func(routineName string, routineReader BackupMetadataReader) {
			defer wg.Done()

			lastRun := routineReader.FindLastRun(ctx)
			if lastRun.FullBackupTime() != nil {
				registry.FinishFull(routineName, *lastRun.FullBackupTime())
			}
			if lastRun.IncrementalBackupTime() != nil {
				registry.FinishIncremental(routineName, *lastRun.IncrementalBackupTime())
			}
		}(routine, reader)
	}

	wg.Wait()
}

// register adds a new backup handler to the registry.
func (r *RunningBackupsRegistryImpl) register(routineName string, job jobType, handler CancelableBackupHandler) {
	r.handlers.Store(makeRegistryKey(routineName, job), handler)
}

// FinishFull remove backup from registry and update last success timestamp.
func (r *RunningBackupsRegistryImpl) FinishFull(routineName string, timestamp time.Time) {
	r.finish(routineName, jobTypeFull, timestamp)
}

// FinishIncremental remove incremental backup from registry and update last success timestamp.
func (r *RunningBackupsRegistryImpl) FinishIncremental(routineName string, timestamp time.Time) {
	r.finish(routineName, jobTypeIncremental, timestamp)
}

func (r *RunningBackupsRegistryImpl) finish(routineName string, job jobType, timestamp time.Time) {
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

	r.remove(routineName, job)
}

// finishWithError remove backup from the registry.
func (r *RunningBackupsRegistryImpl) remove(routineName string, job jobType) {
	r.handlers.Remove(makeRegistryKey(routineName, job))
}

// CurrentStat get the current backup statistics for a routine.
// If there is no backup running, only LastRunTime field will be set.
func (r *RunningBackupsRegistryImpl) CurrentStat(routineName string) *model.CurrentBackups {
	fullBackupHandler, _ := r.handlers.Load(makeRegistryKey(routineName, jobTypeFull))
	incrBackupHandler, _ := r.handlers.Load(makeRegistryKey(routineName, jobTypeIncremental))

	lastRun, found := r.lastSuccessful.Load(routineName)
	if !found {
		lastRun = &model.LastBackupRun{}
	}
	return &model.CurrentBackups{
		Full:        currentBackupStatus(fullBackupHandler),
		Incremental: currentBackupStatus(incrBackupHandler),
		LastRunTime: lastRun,
	}
}

// GetAllCurrentStats all current backups statistics.
// Return only routines that are currently backing up.
func (r *RunningBackupsRegistryImpl) GetAllCurrentStats() map[string]*model.CurrentBackups {
	var routines []string
	r.handlers.Iterate(func(key registryKey, _ CancelableBackupHandler) {
		if !slices.Contains(routines, key.routineName) { // same routine can be stored twice: as full and incr.
			routines = append(routines, key.routineName)
		}
	})

	stats := make(map[string]*model.CurrentBackups, len(routines))
	for _, routineName := range routines {
		stats[routineName] = r.CurrentStat(routineName)
	}

	return stats
}

// Cancel all ongoing backups for a specific routine.
func (r *RunningBackupsRegistryImpl) Cancel(routineName string) {
	for _, job := range []jobType{jobTypeFull, jobTypeIncremental} {
		key := makeRegistryKey(routineName, job)
		if handler, found := r.handlers.Load(key); found {
			handler.Cancel()
		}
	}
}
