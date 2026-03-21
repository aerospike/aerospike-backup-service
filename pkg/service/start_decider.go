package service

import (
	"errors"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

var (
	errFullAlreadyRunning          = errors.New("full backup already in progress")
	errIncrementalNoFullBackup     = errors.New("initial full backup not yet completed")
	errIncrementalFullRunning      = errors.New("full backup in progress")
	errIncrementalAlreadyRunning   = errors.New("incremental backup already in progress")
	errIncrementalFullScheduledNow = errors.New("full backup scheduled at same time")
)

// StartDecider decides whether a backup is allowed to start for a given routine.
//
// Implementations should be pure business logic over StartFacts and routine policy,
// with no mutation, locking, or I/O side effects.
type StartDecider interface {
	// CanStart evaluates admission rules for one start attempt.
	//
	// Returns nil when the start is allowed; otherwise returns a descriptive error
	// that explains why admission must be denied.
	CanStart(
		backupType model.BackupJobType,
		policy *model.BackupPolicy,
		facts StartFacts,
	) error
}

// startDeciderImpl is the single entry point that determines whether a backup can be executed right now.
type startDeciderImpl struct{}

var _ StartDecider = (*startDeciderImpl)(nil)

// StartFacts is the admission-time snapshot consumed by StartDecider.CanStart.
//
// The controller builds this snapshot from registry state plus in-memory pending
// reservations so decision logic can remain deterministic and side-effect free.
type StartFacts struct {
	// FullRunningNow reports whether a full backup is currently running or pending start.
	FullRunningNow bool
	// IncrementalRunningNow reports whether an incremental backup is running or pending start.
	IncrementalRunningNow bool
	// HasCompletedFull reports whether at least one full backup completed in history.
	HasCompletedFull bool
	// FullScheduledNow reports whether this tick is a full-backup cron fire time.
	FullScheduledNow bool
}

// NewStartDecider returns the default StartDecider implementation used by the service.
func NewStartDecider() StartDecider {
	return &startDeciderImpl{}
}

// CanStart dispatches admission checks by backup type.
func (g *startDeciderImpl) CanStart(
	backupType model.BackupJobType,
	policy *model.BackupPolicy,
	facts StartFacts,
) error {
	if backupType == model.BackupJobTypeFull {
		return g.canStartFull(policy, facts)
	}

	return g.canStartIncremental(policy, facts)
}

func (g *startDeciderImpl) canStartFull(
	policy *model.BackupPolicy,
	facts StartFacts,
) error {
	if facts.FullRunningNow && !allowConcurrentFull(policy) {
		return errFullAlreadyRunning
	}

	return nil
}

func (g *startDeciderImpl) canStartIncremental(
	policy *model.BackupPolicy, // pass policy
	facts StartFacts,
) error {
	if !facts.HasCompletedFull {
		return errIncrementalNoFullBackup
	}

	if facts.IncrementalRunningNow && !policy.AllowConcurrentIncremental() {
		return errIncrementalAlreadyRunning
	}

	if facts.FullRunningNow && !policy.AllowConcurrentIncremental() {
		return errIncrementalFullRunning
	}

	if facts.FullScheduledNow && !policy.AllowConcurrentIncremental() {
		return errIncrementalFullScheduledNow
	}

	return nil
}

func allowConcurrentFull(_ *model.BackupPolicy) bool {
	// Future extension point for BackupPolicy.AllowConcurrentFull.
	return false
}
