package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/timeutil"
	"github.com/google/uuid"
)

// StartController coordinates admission for backup starts.
type StartController interface {
	// TryStart attempts to reserve a pending-start slot for the given routine and backup type.
	//
	// If admission is denied, Acquire returns an error wrapped with errBackupSkipped.
	// If admission succeeds, it returns a release callback that MUST be called exactly
	// once to clear the reservation.
	TryStart(
		routine *model.BackupRoutine,
		now time.Time,
		backupType model.BackupType,
	) (release func(), err error)
}

// TokenID identifies a single in-flight admission reservation.
type TokenID = uuid.UUID

type startControllerImpl struct {
	registry     RunningBackupsRegistry
	startDecider StartDecider

	mu sync.Mutex
	// tokenToReservation tracks which reservation each token owns.
	tokenToReservation map[TokenID]reservationKey
	// activeReservations tracks how many pending starts exist by routine+type.
	activeReservations map[reservationKey]int
}

var _ StartController = (*startControllerImpl)(nil)

type reservationKey struct {
	routineName string
	backupType  model.BackupType
}

// NewStartController builds a StartController backed by the provided registry and decision policy.
func NewStartController(
	registry RunningBackupsRegistry,
	policy StartDecider,
) StartController {
	return &startControllerImpl{
		registry:           registry,
		startDecider:       policy,
		tokenToReservation: make(map[TokenID]reservationKey),
		activeReservations: make(map[reservationKey]int),
	}
}

func (a *startControllerImpl) TryStart(
	routine *model.BackupRoutine,
	now time.Time,
	backupType model.BackupType,
) (func(), error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	facts := a.buildStartFacts(routine, now)
	if err := a.startDecider.CanStart(backupType, routine.BackupPolicy, facts); err != nil {
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

// release clears one reservation by token.
//
// This operation is idempotent: unknown or already-released tokens are ignored.
// Multiple reservations for the same routine/type are tracked by a reference count.
func (a *startControllerImpl) release(tokenID TokenID) {
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

// buildStartFacts composes admission facts.
func (a *startControllerImpl) buildStartFacts(routine *model.BackupRoutine, now time.Time) StartFacts {
	state := a.registry.GetRoutineState(routine)
	fullRunning := a.activeReservations[reservationKey{
		routineName: routine.Name,
		backupType:  model.BackupTypeFull,
	}] > 0
	incrRunning := a.activeReservations[reservationKey{
		routineName: routine.Name,
		backupType:  model.BackupTypeIncremental,
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
