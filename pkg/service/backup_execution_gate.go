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

// BackupExecutionGate is the single entry point that determines whether a backup
// can be executed right now.
type BackupExecutionGate struct{}

// BackupStartPolicy contains only business rules that decide whether a backup
// can start for the provided policy facts.
type BackupStartPolicy interface {
	CanStart(
		backupType jobType,
		routine *model.BackupRoutine,
		facts StartFacts,
	) error
}

// StartFacts is a compact, policy-oriented snapshot used by CanStart.
// It deliberately excludes full routine state to keep business checks explicit.
type StartFacts struct {
	FullRunningNow        bool
	IncrementalRunningNow bool
	HasCompletedFull      bool
	FullScheduledNow      bool
}

func NewBackupExecutionGate() *BackupExecutionGate {
	return &BackupExecutionGate{}
}

func (g *BackupExecutionGate) CanStart(
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

func (g *BackupExecutionGate) canStartFull(
	routine *model.BackupRoutine,
	facts StartFacts,
) error {
	if facts.FullRunningNow && !g.allowConcurrentFull(routine) {
		return errFullAlreadyRunning
	}

	return nil
}

func (g *BackupExecutionGate) canStartIncremental(
	routine *model.BackupRoutine,
	facts StartFacts,
) error {
	if !facts.HasCompletedFull {
		return errIncrementalNoFullBackup
	}

	if facts.IncrementalRunningNow && !g.allowConcurrentIncremental(routine) {
		return errIncrementalAlreadyRunning
	}

	if facts.FullRunningNow && g.blockIncrementalWhenFullRunning(routine) {
		return errIncrementalFullRunning
	}

	if facts.FullScheduledNow && g.blockIncrementalAtFullCronFire(routine) {
		return errIncrementalFullScheduledNow
	}

	return nil
}

func (g *BackupExecutionGate) allowConcurrentFull(_ *model.BackupRoutine) bool {
	// Future extension point for BackupPolicy.AllowConcurrentFull.
	return false
}

func (g *BackupExecutionGate) allowConcurrentIncremental(routine *model.BackupRoutine) bool {
	return routine.BackupPolicy.AllowConcurrentIncremental()
}

func (g *BackupExecutionGate) blockIncrementalWhenFullRunning(routine *model.BackupRoutine) bool {
	return !g.allowConcurrentIncremental(routine)
}

func (g *BackupExecutionGate) blockIncrementalAtFullCronFire(routine *model.BackupRoutine) bool {
	return !g.allowConcurrentIncremental(routine)
}
