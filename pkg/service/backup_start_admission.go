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
	Acquire(
		routine *model.BackupRoutine,
		backupType jobType,
		now time.Time,
	) (release func(), err error)
}

// TokenID is an opaque reservation handle used internally by start admission.
type TokenID = uuid.UUID

type startAdmission struct {
	registry RunningBackupsRegistry
	policy   BackupStartPolicy

	mu sync.Mutex
	// tokenToReservation tracks which reservation each token owns.
	tokenToReservation map[TokenID]reservationKey
	// activeReservations tracks how many pending starts exist by routine+type.
	activeReservations map[reservationKey]int
}

type reservationKey struct {
	routineName string
	backupType  jobType
}

func NewBackupStartAdmission(
	registry RunningBackupsRegistry,
	policy BackupStartPolicy,
) BackupStartAdmission {
	return &startAdmission{
		registry:           registry,
		policy:             policy,
		tokenToReservation: make(map[TokenID]reservationKey),
		activeReservations: make(map[reservationKey]int),
	}
}

func (a *startAdmission) Acquire(
	routine *model.BackupRoutine,
	backupType jobType,
	now time.Time,
) (func(), error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	facts := a.buildStartFactsLocked(routine, now)
	if err := a.policy.CanStart(backupType, routine, facts); err != nil {
		return nil, fmt.Errorf("%w: %w", errBackupSkipped, err)
	}

	tokenID := uuid.New()
	key := reservationKey{
		routineName: routine.Name,
		backupType:  backupType,
	}
	a.tokenToReservation[tokenID] = key
	a.activeReservations[key]++

	return func() {
		a.release(tokenID)
	}, nil
}

// release is idempotent: unknown/already-released token is a no-op.
func (a *startAdmission) release(tokenID TokenID) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key, ok := a.tokenToReservation[tokenID]
	if !ok {
		return
	}
	delete(a.tokenToReservation, tokenID)

	count := a.activeReservations[key]
	if count <= 1 {
		delete(a.activeReservations, key)
		return
	}
	a.activeReservations[key] = count - 1
}

func (a *startAdmission) buildStartFactsLocked(routine *model.BackupRoutine, now time.Time) StartFacts {
	state := a.registry.GetRoutineState(routine)
	fullRunning := a.activeReservations[reservationKey{
		routineName: routine.Name,
		backupType:  jobTypeFull,
	}] > 0
	incrRunning := a.activeReservations[reservationKey{
		routineName: routine.Name,
		backupType:  jobTypeIncremental,
	}] > 0

	return StartFacts{
		// Reservation is treated as running for admission checks.
		FullRunningNow:        fullRunning,
		IncrementalRunningNow: incrRunning,
		// History still comes from registry.
		HasCompletedFull: !state.LastRunTime.NoFullBackup(),
		FullScheduledNow: timeutil.IsCronFireTime(routine.IntervalCron, now),
	}
}
