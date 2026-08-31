package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
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

	require.NoError(t, applier.ApplyNewConfig(t.Context()))
}

func TestConfigApplier_ApplyNewConfig_ReschedulesInvalidatedRoutine(t *testing.T) {
	ctrl := gomock.NewController(t)

	cfg := model.NewConfig()
	require.NoError(t, cfg.AddRoutine(&model.BackupRoutine{
		Name:         "routine-1",
		IntervalCron: "0 0 * * * *",
		Timezone:     time.UTC,
	}))
	cfg.PopInvalidatedRoutineNames()
	cfg.InvalidateRoutines([]string{"routine-1"})

	scheduler := NewMockJobScheduler(ctrl)
	scheduler.EXPECT().DeleteJob(jobKey("routine-1", model.BackupTypeFull)).Return(nil)
	scheduler.EXPECT().DeleteJob(jobKey("routine-1", model.BackupTypeIncremental)).Return(nil)
	scheduler.EXPECT().ScheduleJob(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	syncDone := make(chan struct{})
	registry := NewMockBackupStateRegistry(ctrl)
	registry.EXPECT().SynchroniseBackupHistory(gomock.Any(), gomock.Any()).
		Do(func(context.Context, []*model.BackupRoutine) { close(syncDone) })

	applier := NewConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		registry,
		cfg,
	)

	require.NoError(t, applier.ApplyNewConfig(t.Context()))
	waitAsyncDone(t, syncDone, "backup history sync")
}

func TestConfigApplier_ApplyNewConfig_SkipsDeletedRoutine(t *testing.T) {
	ctrl := gomock.NewController(t)

	cfg := model.NewConfig()
	cfg.InvalidateRoutines([]string{"removed-routine"})

	scheduler := NewMockJobScheduler(ctrl)
	scheduler.EXPECT().DeleteJob(jobKey("removed-routine", model.BackupTypeFull)).Return(nil)
	scheduler.EXPECT().DeleteJob(jobKey("removed-routine", model.BackupTypeIncremental)).Return(nil)

	syncDone := make(chan struct{})
	registry := NewMockBackupStateRegistry(ctrl)
	registry.EXPECT().SynchroniseBackupHistory(gomock.Any(), gomock.Len(0)).
		Do(func(context.Context, []*model.BackupRoutine) { close(syncDone) })

	applier := NewConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		registry,
		cfg,
	)

	require.NoError(t, applier.ApplyNewConfig(t.Context()))
	waitAsyncDone(t, syncDone, "backup history sync")
}

func TestConfigApplier_ApplyNewConfig_ScheduleError(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := model.NewConfig()
	require.NoError(t, cfg.AddRoutine(&model.BackupRoutine{
		Name:         "routine-1",
		IntervalCron: "not-a-cron",
		Timezone:     time.UTC,
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

	err := applier.ApplyNewConfig(t.Context())
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to schedule periodic backups")
}

func TestConfigApplier_ApplyNewConfig_ScheduleJobError(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := model.NewConfig()
	require.NoError(t, cfg.AddRoutine(&model.BackupRoutine{
		Name:         "routine-1",
		IntervalCron: "0 0 * * * *",
		Timezone:     time.UTC,
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

	err := applier.ApplyNewConfig(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, scheduleErr)
}
