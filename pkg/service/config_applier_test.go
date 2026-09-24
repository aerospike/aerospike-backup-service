package service

import (
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/quartz"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestConfigApplier_ApplyNewConfig_NoInvalidations(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := model.NewConfig()
	scheduler := NewMockJobScheduler(ctrl)

	applier := NewConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		NewMockBackupStateRegistry(ctrl),
		cfg,
	)

	require.NoError(t, applier.ApplyNewConfig())
}

func TestConfigApplier_ApplyNewConfig_ReschedulesInvalidatedRoutine(t *testing.T) {
	ctrl := gomock.NewController(t)

	cfg := model.NewConfig()
	require.NoError(t, cfg.AddRoutine(&model.BackupRoutine{
		Name:         "routine-1",
		IntervalCron: "0 0 * * * *",
		Timezone:     model.NewServiceLocation("", nil),
	}))
	cfg.PopInvalidatedRoutineNames()
	cfg.InvalidateRoutines([]string{"routine-1"})

	scheduler := NewMockJobScheduler(ctrl)
	scheduler.EXPECT().DeleteJob(jobKey("routine-1", model.BackupTypeFull)).Return(nil)
	scheduler.EXPECT().DeleteJob(jobKey("routine-1", model.BackupTypeIncremental)).Return(nil)
	scheduler.EXPECT().ScheduleJob(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	registry := NewMockBackupStateRegistry(ctrl)
	registry.EXPECT().RequestHistorySync(gomock.Len(1))

	applier := NewConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		registry,
		cfg,
	)

	require.NoError(t, applier.ApplyNewConfig())
}

func TestConfigApplier_ApplyNewConfig_UnschedulesDeletedRoutine(t *testing.T) {
	ctrl := gomock.NewController(t)

	cfg := model.NewConfig()
	cfg.InvalidateRoutines([]string{"removed-routine"})

	adHocJob := adhocKey("removed-routine", model.BackupTypeFull, 1)
	scheduler := NewMockJobScheduler(ctrl)
	scheduler.EXPECT().DeleteJob(jobKey("removed-routine", model.BackupTypeFull)).Return(nil)
	scheduler.EXPECT().DeleteJob(jobKey("removed-routine", model.BackupTypeIncremental)).Return(nil)
	scheduler.EXPECT().GetJobKeys(gomock.Any()).Return([]*quartz.JobKey{adHocJob}, nil)
	scheduler.EXPECT().DeleteJob(adHocJob).Return(nil)

	registry := NewMockBackupStateRegistry(ctrl)
	registry.EXPECT().RequestHistorySync(gomock.Len(0))

	applier := NewConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		registry,
		cfg,
	)

	require.NoError(t, applier.ApplyNewConfig())
}

func TestConfigApplier_ApplyNewConfig_ScheduleError(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := model.NewConfig()
	require.NoError(t, cfg.AddRoutine(&model.BackupRoutine{
		Name:         "routine-1",
		IntervalCron: "not-a-cron",
		Timezone:     model.NewServiceLocation("", nil),
	}))
	cfg.PopInvalidatedRoutineNames()
	cfg.InvalidateRoutines([]string{"routine-1"})

	scheduler := NewMockJobScheduler(ctrl)
	scheduler.EXPECT().DeleteJob(gomock.Any()).Return(nil).Times(2)

	applier := NewConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		NewMockBackupStateRegistry(ctrl),
		cfg,
	)

	err := applier.ApplyNewConfig()
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to schedule periodic backups")
}

func TestConfigApplier_ApplyNewConfig_ScheduleJobError(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := model.NewConfig()
	require.NoError(t, cfg.AddRoutine(&model.BackupRoutine{
		Name:         "routine-1",
		IntervalCron: "0 0 * * * *",
		Timezone:     model.NewServiceLocation("", nil),
	}))
	cfg.PopInvalidatedRoutineNames()
	cfg.InvalidateRoutines([]string{"routine-1"})

	scheduleErr := errors.New("schedule failed")
	scheduler := NewMockJobScheduler(ctrl)
	scheduler.EXPECT().DeleteJob(gomock.Any()).Return(nil).Times(2)
	scheduler.EXPECT().ScheduleJob(gomock.Any(), gomock.Any()).Return(scheduleErr)

	applier := NewConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		NewMockBackupStateRegistry(ctrl),
		cfg,
	)

	err := applier.ApplyNewConfig()
	require.Error(t, err)
	require.ErrorIs(t, err, scheduleErr)
}
