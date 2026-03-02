package service

import (
	"errors"
	"fmt"
	"time"

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

// backupExecutionOverlapPolicy encapsulates overlap constraints between backup types.
// Keep all overlap decisions here so future flags (for example AllowConcurrentFull)
// can be introduced without changing orchestration flow.
type backupExecutionOverlapPolicy interface {
	allowConcurrentFull(routine *model.BackupRoutine) bool
	allowConcurrentIncremental(routine *model.BackupRoutine) bool
	blockIncrementalWhenFullRunning(routine *model.BackupRoutine) bool
	blockIncrementalAtFullCronFire(routine *model.BackupRoutine) bool
}

type routinePolicyOverlap struct{}

func (routinePolicyOverlap) allowConcurrentFull(_ *model.BackupRoutine) bool {
	// Future extension point for BackupPolicy.AllowConcurrentFull.
	return false
}

func (routinePolicyOverlap) allowConcurrentIncremental(routine *model.BackupRoutine) bool {
	return routine.BackupPolicy.AllowConcurrentIncremental()
}

func (p routinePolicyOverlap) blockIncrementalWhenFullRunning(routine *model.BackupRoutine) bool {
	return !p.allowConcurrentIncremental(routine)
}

func (p routinePolicyOverlap) blockIncrementalAtFullCronFire(routine *model.BackupRoutine) bool {
	return !p.allowConcurrentIncremental(routine)
}

// BackupExecutionGate is the single entry point that determines whether a backup
// can be executed right now.
type BackupExecutionGate struct {
	policy backupExecutionOverlapPolicy
}

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
	Now                   time.Time
}

func NewBackupExecutionGate(policy backupExecutionOverlapPolicy) *BackupExecutionGate {
	if policy == nil {
		policy = routinePolicyOverlap{}
	}

	return &BackupExecutionGate{policy: policy}
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

func (g *BackupExecutionGate) canStartFull(routine *model.BackupRoutine, facts StartFacts) error {
	if facts.FullRunningNow && !g.policy.allowConcurrentFull(routine) {
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

	if facts.IncrementalRunningNow && !g.policy.allowConcurrentIncremental(routine) {
		return errIncrementalAlreadyRunning
	}

	if facts.FullRunningNow && g.policy.blockIncrementalWhenFullRunning(routine) {
		return errIncrementalFullRunning
	}

	if facts.FullScheduledNow && g.policy.blockIncrementalAtFullCronFire(routine) {
		return errIncrementalFullScheduledNow
	}

	return nil
}
