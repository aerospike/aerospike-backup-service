package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestStartControllerAcquire_DeniesConcurrentAcquireForSameRoutineType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)
	routine := testRoutine()
	registry := NewMockRunningBackupsRegistry(ctrl)
	registry.EXPECT().GetRoutineState(gomock.Any()).Return(
		model.RoutineState{LastRunTime: model.NewNoBackupTime()},
	).Times(3)

	controller := NewStartController(
		registry,
		NewStartDecider(),
	)

	release, err := controller.TryStart(routine, now, model.BackupTypeFull)
	require.NoError(t, err)
	require.NotNil(t, release)

	secondRelease, err := controller.TryStart(routine, now, model.BackupTypeFull)
	assert.Nil(t, secondRelease)
	require.ErrorIs(t, err, errFullAlreadyRunning)

	release()

	thirdRelease, err := controller.TryStart(routine, now, model.BackupTypeFull)
	require.NoError(t, err)
	require.NotNil(t, thirdRelease)
	thirdRelease()
}

func TestStartControllerAcquire_DeniesIncrementalWhenFullIsReserved(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)
	routine := testRoutine()
	registry := NewMockRunningBackupsRegistry(ctrl)
	registry.EXPECT().GetRoutineState(gomock.Any()).Return(
		model.RoutineState{LastRunTime: model.NewFullBackupTime(now.Add(-24 * time.Hour))},
	).Times(3)

	controller := NewStartController(
		registry,
		NewStartDecider(),
	)

	releaseFull, err := controller.TryStart(routine, now, model.BackupTypeFull)
	require.NoError(t, err)
	require.NotNil(t, releaseFull)

	releaseIncremental, err := controller.TryStart(routine, now, model.BackupTypeIncremental)
	assert.Nil(t, releaseIncremental)
	require.ErrorIs(t, err, errIncrementalFullRunning)

	releaseFull()

	releaseIncremental, err = controller.TryStart(routine, now, model.BackupTypeIncremental)
	require.NoError(t, err)
	require.NotNil(t, releaseIncremental)
	releaseIncremental()
}

func TestStartControllerRelease_IsIdempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)
	routine := testRoutine()
	registry := NewMockRunningBackupsRegistry(ctrl)
	registry.EXPECT().GetRoutineState(gomock.Any()).Return(
		model.RoutineState{LastRunTime: model.NewNoBackupTime()},
	).Times(2)

	controller := NewStartController(
		registry,
		NewStartDecider(),
	)

	release, err := controller.TryStart(routine, now, model.BackupTypeFull)
	require.NoError(t, err)
	require.NotNil(t, release)

	require.NotPanics(t, func() {
		release()
		release()
	})

	releaseAgain, err := controller.TryStart(routine, now, model.BackupTypeFull)
	require.NoError(t, err)
	require.NotNil(t, releaseAgain)
	releaseAgain()
}

func TestStartControllerRelease_TracksReservationCountByToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)
	routine := testRoutine()
	routine.BackupPolicy.ConcurrentIncremental = ptr.Of(true)
	registry := NewMockRunningBackupsRegistry(ctrl)
	registry.EXPECT().GetRoutineState(gomock.Any()).Return(
		model.RoutineState{LastRunTime: model.NewFullBackupTime(now.Add(-24 * time.Hour))},
	).AnyTimes()

	controller := NewStartController(
		registry,
		NewStartDecider(),
	)

	releaseOne, err := controller.TryStart(routine, now, model.BackupTypeIncremental)
	require.NoError(t, err)
	releaseTwo, err := controller.TryStart(routine, now, model.BackupTypeIncremental)
	require.NoError(t, err)

	impl := controller.(*startControllerImpl)
	key := reservationKey{
		routineName: routine.Name,
		backupType:  model.BackupTypeIncremental,
	}
	require.Equal(t, 2, impl.activeReservations[key])
	require.Len(t, impl.tokenToReservation, 2)

	releaseOne()
	require.Equal(t, 1, impl.activeReservations[key])
	require.Len(t, impl.tokenToReservation, 1)

	releaseOne() // idempotent second release for same token
	require.Equal(t, 1, impl.activeReservations[key])
	require.Len(t, impl.tokenToReservation, 1)

	releaseTwo()
	_, ok := impl.activeReservations[key]
	require.False(t, ok)
	require.Empty(t, impl.tokenToReservation)
}

func TestStartController_HasBackupRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC)
	routine := testRoutine()
	registry := NewMockRunningBackupsRegistry(ctrl)
	registry.EXPECT().GetRoutineState(gomock.Any()).Return(
		model.RoutineState{LastRunTime: model.NewNoBackupTime()},
	).AnyTimes()

	controller := NewStartController(registry, NewStartDecider())

	require.False(t, controller.HasBackupRunning(routine))

	release, err := controller.TryStart(routine, now, model.BackupTypeFull)
	require.NoError(t, err)
	require.True(t, controller.HasBackupRunning(routine))

	release()
	require.False(t, controller.HasBackupRunning(routine))
}
