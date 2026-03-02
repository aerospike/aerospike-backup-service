package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/timeutil"
	"github.com/google/uuid"
)

// BackupStartAdmission owns only concurrency admission:
// atomically check policy against running+pending state and reserve a pending slot.
type BackupStartAdmission interface {
	TryAcquire(
		routine *model.BackupRoutine,
		backupType jobType,
		now time.Time,
	) (StartToken, error)
	Release(token StartToken)
}

// StartToken is an opaque reservation handle returned by TryAcquire.
// It must be passed back to Release.
type StartToken = uuid.UUID

type startAdmission struct {
	registry RunningBackupsRegistry
	policy   BackupStartPolicy

	mu     sync.Mutex
	tokens map[StartToken]reservation
}

type reservation struct {
	routineName string
	backupType  jobType
}

func NewBackupStartAdmission(
	registry RunningBackupsRegistry,
	policy BackupStartPolicy,
) BackupStartAdmission {
	if policy == nil {
		policy = NewBackupExecutionGate(nil)
	}

	return &startAdmission{
		registry: registry,
		policy:   policy,
		tokens:   make(map[uuid.UUID]reservation),
	}
}

func (a *startAdmission) TryAcquire(
	routine *model.BackupRoutine,
	backupType jobType,
	now time.Time,
) (StartToken, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	facts := a.buildStartFactsLocked(routine, now)
	if err := a.policy.CanStart(backupType, routine, facts); err != nil {
		return StartToken{}, fmt.Errorf("%w: %w", errBackupSkipped, err)
	}

	id := uuid.New()
	a.tokens[id] = reservation{
		routineName: routine.Name,
		backupType:  backupType,
	}

	return id, nil
}

// Release is idempotent: unknown/already-released token is a no-op.
func (a *startAdmission) Release(token StartToken) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.tokens, token)
}

func (a *startAdmission) buildStartFactsLocked(routine *model.BackupRoutine, now time.Time) StartFacts {
	state := a.registry.GetRoutineState(routine)
	fullRunning := false
	incrRunning := false
	for _, token := range a.tokens {
		if token.routineName != routine.Name {
			continue
		}
		if token.backupType == jobTypeFull {
			fullRunning = true
		} else if token.backupType == jobTypeIncremental {
			incrRunning = true
		}
		if fullRunning && incrRunning {
			break
		}
	}

	return StartFacts{
		// Reservation is treated as running for admission checks.
		FullRunningNow:        fullRunning,
		IncrementalRunningNow: incrRunning,
		// History still comes from registry.
		HasCompletedFull: !state.LastRunTime.NoFullBackup(),
		FullScheduledNow: timeutil.IsCronFireTime(routine.IntervalCron, now),
		Now:              now,
	}
}
