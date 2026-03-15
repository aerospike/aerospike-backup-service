package service

import (
	"errors"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

var (
	errFullAlreadyRunning           = errors.New("full backup already in progress")
	errIncrementalNoFullBackup      = errors.New("initial full backup not yet completed")
	errIncrementalFullRunning       = errors.New("full backup in progress")
	errIncrementalAlreadyRunning    = errors.New("incremental backup already in progress")
	errIncrementalFullScheduledNow  = errors.New("full backup scheduled at same time")
	errUnsupportedBackupTypeToStart = errors.New("unsupported backup type")
)

// StartDecider contains only business rules that decide whether a backup can start for the provided facts.
type StartDecider interface {
	CanStart(
		backupType jobType,
		routine *model.BackupRoutine,
		facts StartFacts,
	) error
}

// startDeciderImpl is the single entry point that determines whether a backup can be executed right now.
type startDeciderImpl struct{}

var _ StartDecider = (*startDeciderImpl)(nil)

// StartFacts is a compact snapshot used by CanStart.
type StartFacts struct {
	FullRunningNow        bool
	IncrementalRunningNow bool
	HasCompletedFull      bool
	FullScheduledNow      bool
}

func NewStartDecider() StartDecider {
	return &startDeciderImpl{}
}

func (g *startDeciderImpl) CanStart(
	backupType jobType,
	routine *model.BackupRoutine,
	facts StartFacts,
) error {
	switch backupType {
	case jobTypeFull:
		return g.canStartFull(routine, facts)
	case jobTypeIncremental:
		return g.canStartIncremental(routine, facts)
	default:
		return fmt.Errorf("%w: %s", errUnsupportedBackupTypeToStart, backupType)
	}
}

func (g *startDeciderImpl) canStartFull(
	routine *model.BackupRoutine,
	facts StartFacts,
) error {
	if facts.FullRunningNow && !allowConcurrentFull(routine) {
		return errFullAlreadyRunning
	}

	return nil
}

func (g *startDeciderImpl) canStartIncremental(
	routine *model.BackupRoutine,
	facts StartFacts,
) error {
	if !facts.HasCompletedFull {
		return errIncrementalNoFullBackup
	}

	if facts.IncrementalRunningNow && !routine.BackupPolicy.AllowConcurrentIncremental() {
		return errIncrementalAlreadyRunning
	}

	if facts.FullRunningNow && !routine.BackupPolicy.AllowConcurrentIncremental() {
		return errIncrementalFullRunning
	}

	if facts.FullScheduledNow && !routine.BackupPolicy.AllowConcurrentIncremental() {
		return errIncrementalFullScheduledNow
	}

	return nil
}

func allowConcurrentFull(_ *model.BackupRoutine) bool {
	// Future extension point for BackupPolicy.AllowConcurrentFull.
	return false
}
