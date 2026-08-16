package service

import (
	"context"
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDefaultConfigApplier_ApplyNewConfig_NoInvalidations(t *testing.T) {
	cfg := model.NewConfig()
	scheduler := new(MockScheduler)

	applier := NewDefaultConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		NewMockRunningBackupsRegistry(gomock.NewController(t)),
		cfg,
	)

	require.NoError(t, applier.ApplyNewConfig(t.Context()))
	scheduler.AssertNotCalled(t, "DeleteJob", mock.Anything)
	scheduler.AssertNotCalled(t, "ScheduleJob", mock.Anything, mock.Anything)
}

func TestDefaultConfigApplier_ApplyNewConfig_ReschedulesInvalidatedRoutine(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := model.NewConfig()
	require.NoError(t, cfg.AddRoutine(&model.BackupRoutine{
		Name:         "routine-1",
		IntervalCron: "0 0 * * * *",
	}))
	cfg.PopInvalidatedRoutineNames()
	cfg.InvalidateRoutines([]string{"routine-1"})

	scheduler := new(MockScheduler)
	scheduler.On("DeleteJob", jobKey("routine-1", model.BackupTypeFull)).Return(nil)
	scheduler.On("DeleteJob", jobKey("routine-1", model.BackupTypeIncremental)).Return(nil)
	scheduler.On("ScheduleJob", mock.Anything, mock.Anything).Return(nil)

	registry := NewMockRunningBackupsRegistry(ctrl)
	syncDone := make(chan struct{})
	registry.EXPECT().SynchroniseBackupHistory(gomock.Any(), gomock.Any()).
		Do(func(context.Context, []*model.BackupRoutine) { close(syncDone) })

	applier := NewDefaultConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		registry,
		cfg,
	)

	require.NoError(t, applier.ApplyNewConfig(t.Context()))
	scheduler.AssertNumberOfCalls(t, "DeleteJob", 2)
	scheduler.AssertNumberOfCalls(t, "ScheduleJob", 1)
	<-syncDone
}

func TestDefaultConfigApplier_ApplyNewConfig_SkipsDeletedRoutine(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := model.NewConfig()
	cfg.InvalidateRoutines([]string{"removed-routine"})

	scheduler := new(MockScheduler)
	scheduler.On("DeleteJob", jobKey("removed-routine", model.BackupTypeFull)).Return(nil)
	scheduler.On("DeleteJob", jobKey("removed-routine", model.BackupTypeIncremental)).Return(nil)

	registry := NewMockRunningBackupsRegistry(ctrl)
	syncDone := make(chan struct{})
	registry.EXPECT().SynchroniseBackupHistory(gomock.Any(), gomock.Len(0)).
		Do(func(context.Context, []*model.BackupRoutine) { close(syncDone) })

	applier := NewDefaultConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		registry,
		cfg,
	)

	require.NoError(t, applier.ApplyNewConfig(t.Context()))
	scheduler.AssertNotCalled(t, "ScheduleJob", mock.Anything, mock.Anything)
	<-syncDone
}

func TestDefaultConfigApplier_ApplyNewConfig_ScheduleError(t *testing.T) {
	cfg := model.NewConfig()
	require.NoError(t, cfg.AddRoutine(&model.BackupRoutine{
		Name:         "routine-1",
		IntervalCron: "not-a-cron",
	}))
	cfg.PopInvalidatedRoutineNames()
	cfg.InvalidateRoutines([]string{"routine-1"})

	scheduler := new(MockScheduler)
	scheduler.On("DeleteJob", mock.Anything).Return(nil)

	applier := NewDefaultConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		NewMockRunningBackupsRegistry(gomock.NewController(t)),
		cfg,
	)

	err := applier.ApplyNewConfig(t.Context())
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to schedule periodic backups")
}

func TestDefaultConfigApplier_ApplyNewConfig_ScheduleJobError(t *testing.T) {
	cfg := model.NewConfig()
	require.NoError(t, cfg.AddRoutine(&model.BackupRoutine{
		Name:         "routine-1",
		IntervalCron: "0 0 * * * *",
	}))
	cfg.PopInvalidatedRoutineNames()
	cfg.InvalidateRoutines([]string{"routine-1"})

	scheduleErr := errors.New("schedule failed")
	scheduler := new(MockScheduler)
	scheduler.On("DeleteJob", mock.Anything).Return(nil)
	scheduler.On("ScheduleJob", mock.Anything, mock.Anything).Return(scheduleErr)

	applier := NewDefaultConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		NewMockRunningBackupsRegistry(gomock.NewController(t)),
		cfg,
	)

	err := applier.ApplyNewConfig(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, scheduleErr)
}
