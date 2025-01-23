package service

import (
	"slices"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
)

type RunningBackupsRegistry interface {
	add(string, jobType, CancelableBackupHandler)
	contains(string, jobType) bool
	CurrentStat(string) *model.CurrentBackups
	Cancel(string)
	finishWithError(routineName string, job jobType)
	FinishFull(routineName string, time time.Time)
	FinishIncremental(routineName string, time time.Time)
	getRoutines() []string
}

type key struct {
	routineName string
	job         jobType
}

type RunningBackupsRegistryImpl struct {
	handlers       *util.SafeMap[key, CancelableBackupHandler]
	lastSuccessful *util.SafeMap[string, *model.LastBackupRun]
}

func NewRunningBackupsRegistry() RunningBackupsRegistry {
	return &RunningBackupsRegistryImpl{
		handlers:       util.NewSafeMap[key, CancelableBackupHandler](),
		lastSuccessful: util.NewSafeMap[string, *model.LastBackupRun](),
	}
}

func (r *RunningBackupsRegistryImpl) add(routineName string, job jobType, handler CancelableBackupHandler) {
	k := key{routineName: routineName, job: job}
	r.handlers.Store(k, handler)
}

func (r *RunningBackupsRegistryImpl) FinishFull(routineName string, time time.Time) {
	// update last backup run time.
	r.lastSuccessful.ApplyOrCreate(
		routineName,
		func(lastBackupRun *model.LastBackupRun) {
			lastBackupRun.SetFullBackupTime(&time)
		},
		model.NewLastBackupRun(&time, nil), // it was first backup, always full.
	)

	k := key{routineName: routineName, job: jobTypeFull}
	r.handlers.Remove(k)
}

func (r *RunningBackupsRegistryImpl) FinishIncremental(routineName string, time time.Time) {
	// update last backup run time. Incremental can be finished only when full already exists.
	r.lastSuccessful.Apply(
		routineName,
		func(lastBackupRun *model.LastBackupRun) {
			lastBackupRun.SetIncrementalBackupTime(&time)
		},
	)

	k := key{routineName: routineName, job: jobTypeIncremental}
	r.handlers.Remove(k)
}

func (r *RunningBackupsRegistryImpl) finishWithError(routineName string, job jobType) {
	k := key{routineName: routineName, job: job}
	r.handlers.Remove(k)
}

func (r *RunningBackupsRegistryImpl) contains(routineName string, job jobType) bool {
	k := key{routineName: routineName, job: job}
	_, exists := r.handlers.Load(k)
	return exists
}

func (r *RunningBackupsRegistryImpl) CurrentStat(routineName string) *model.CurrentBackups {
	fullBackupHandler, _ := r.handlers.Load(key{routineName: routineName, job: jobTypeFull})
	incrBackupHandler, _ := r.handlers.Load(key{routineName: routineName, job: jobTypeIncremental})

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

func (r *RunningBackupsRegistryImpl) Cancel(routineName string) {
	fullBackupHandler, found := r.handlers.Load(key{routineName: routineName, job: jobTypeFull})
	if found {
		fullBackupHandler.Cancel()
	}
	incrBackupHandler, found := r.handlers.Load(key{routineName: routineName, job: jobTypeIncremental})
	if found {
		incrBackupHandler.Cancel()
	}
}

func (r *RunningBackupsRegistryImpl) getRoutines() []string {
	var routines []string
	r.handlers.Iterate(func(key key, _ CancelableBackupHandler) {
		if !slices.Contains(routines, key.routineName) {
			routines = append(routines, key.routineName)
		}
	})

	return routines
}
