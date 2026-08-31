package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/timeutil"
	"github.com/google/uuid"
)

// StartController decides whether a backup may start and holds the reservation until the run ends.
type StartController interface {
	// TryStart asks permission to start a backup of the given type for the routine.
	//
	// If the start is denied, it returns an error wrapping errBackupSkipped.
	// If it is allowed, it returns a release callback that MUST be called exactly
	// once to clear the reservation.
	TryStart(
		routine *model.BackupRoutine,
		now time.Time,
		backupType model.BackupType,
	) (release func(), err error)
	// HasBackupRunning reports whether a full or incremental backup is active for the
	// routine, including admitted starts not yet visible in the running-backups registry.
	HasBackupRunning(routine *model.BackupRoutine) bool
}

// TokenID identifies a single in-flight admission reservation.
type TokenID = uuid.UUID

type startController struct {
	registry     BackupStateRegistry
	startDecider StartDecider

	mu sync.Mutex
	// tokenToReservation tracks which reservation each token owns.
	tokenToReservation map[TokenID]reservationKey
	// activeReservations tracks how many pending starts exist by routine+type.
	activeReservations map[reservationKey]int
}

var _ StartController = (*startController)(nil)

type reservationKey struct {
	routineName string
	backupType  model.BackupType
}

// NewStartController returns a StartController.
func NewStartController(
	registry BackupStateRegistry,
	policy StartDecider,
) StartController {
	return &startController{
		registry:           registry,
		startDecider:       policy,
		tokenToReservation: make(map[TokenID]reservationKey),
		activeReservations: make(map[reservationKey]int),
	}
}

func (s *startController) TryStart(
	routine *model.BackupRoutine,
	now time.Time,
	backupType model.BackupType,
) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	facts := s.buildStartFacts(routine, now)
	if err := s.startDecider.CanStart(backupType, routine.BackupPolicy, facts); err != nil {
		return nil, fmt.Errorf("%w: %w", errBackupSkipped, err)
	}

	tokenID := uuid.New()
	key := reservationKey{
		routineName: routine.Name,
		backupType:  backupType,
	}
	s.tokenToReservation[tokenID] = key
	s.activeReservations[key]++

	return func() {
		s.release(tokenID)
	}, nil
}

// HasBackupRunning reports whether a full or incremental backup is active for the routine.
func (s *startController) HasBackupRunning(routine *model.BackupRoutine) bool {
	state := s.registry.GetRoutineState(routine)
	if state.Full != nil || state.Incremental != nil {
		return true
	}

	return s.hasPendingStart(routine.Name, model.BackupTypeFull) ||
		s.hasPendingStart(routine.Name, model.BackupTypeIncremental)
}

func (s *startController) hasPendingStart(routineName string, backupType model.BackupType) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.hasPendingStartLocked(reservationKey{
		routineName: routineName,
		backupType:  backupType,
	})
}

func (s *startController) hasPendingStartLocked(key reservationKey) bool {
	return s.activeReservations[key] > 0
}

// release clears one reservation by token.
//
// This operation is idempotent: unknown or already-released tokens are ignored.
// Multiple reservations for the same routine/type are tracked by a reference count.
func (s *startController) release(tokenID TokenID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, ok := s.tokenToReservation[tokenID]
	if !ok {
		return
	}
	delete(s.tokenToReservation, tokenID)

	count := s.activeReservations[key]
	if count <= 1 {
		delete(s.activeReservations, key)
		return
	}
	s.activeReservations[key] = count - 1
}

// buildStartFacts composes admission facts.
func (s *startController) buildStartFacts(routine *model.BackupRoutine, now time.Time) StartFacts {
	state := s.registry.GetRoutineState(routine)
	fullRunning := s.hasPendingStartLocked(reservationKey{
		routineName: routine.Name,
		backupType:  model.BackupTypeFull,
	})
	incrRunning := s.hasPendingStartLocked(reservationKey{
		routineName: routine.Name,
		backupType:  model.BackupTypeIncremental,
	})

	return StartFacts{
		// Reservation is treated as running for admission checks.
		FullRunningNow:        fullRunning,
		IncrementalRunningNow: incrRunning,
		// History still comes from registry.
		HasCompletedFull: !state.LastRunTime.NoFullBackup(),
		FullScheduledNow: timeutil.IsCronFireTime(routine.IntervalCron, now, routine.Timezone),
	}
}
