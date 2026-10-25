package service

import (
	"context"
	"errors"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
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
		noopNamespaceValidator(ctrl),
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
		noopNamespaceValidator(ctrl),
		cfg,
	)

	require.NoError(t, applier.ApplyNewConfig(t.Context()))
}

func TestConfigApplier_ApplyNewConfig_SkipsDeletedRoutine(t *testing.T) {
	ctrl := gomock.NewController(t)

	cfg := model.NewConfig()
	cfg.InvalidateRoutines([]string{"removed-routine"})

	scheduler := NewMockJobScheduler(ctrl)
	scheduler.EXPECT().DeleteJob(jobKey("removed-routine", model.BackupTypeFull)).Return(nil)
	scheduler.EXPECT().DeleteJob(jobKey("removed-routine", model.BackupTypeIncremental)).Return(nil)

	registry := NewMockBackupStateRegistry(ctrl)
	registry.EXPECT().RequestHistorySync(gomock.Len(0))

	applier := NewConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		registry,
		noopNamespaceValidator(ctrl),
		cfg,
	)

	require.NoError(t, applier.ApplyNewConfig(t.Context()))
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
		noopNamespaceValidator(ctrl),
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
		noopNamespaceValidator(ctrl),
		cfg,
	)

	err := applier.ApplyNewConfig(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, scheduleErr)
}

// Namespace validation is advisory and scoped: only the routines that were invalidated and
// still exist are checked, after they have been scheduled.
func TestConfigApplier_ApplyNewConfig_ValidatesAppliedRoutinesOnly(t *testing.T) {
	ctrl := gomock.NewController(t)

	cfg := model.NewConfig()
	for _, name := range []string{"changed", "untouched"} {
		require.NoError(t, cfg.AddRoutine(&model.BackupRoutine{
			Name:         name,
			IntervalCron: "0 0 * * * *",
			Timezone:     model.NewServiceLocation("", nil),
		}))
	}
	cfg.PopInvalidatedRoutineNames()
	cfg.InvalidateRoutines([]string{"changed", "removed"})

	scheduler := NewMockJobScheduler(ctrl)
	scheduler.EXPECT().DeleteJob(gomock.Any()).Return(nil).Times(4)
	scheduler.EXPECT().ScheduleJob(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	registry := NewMockBackupStateRegistry(ctrl)
	registry.EXPECT().RequestHistorySync([]string{"changed"})

	nsValidator := aerospike.NewMockNamespaceValidator(ctrl)
	nsValidator.EXPECT().Validate(gomock.Any(), gomock.Any()).Do(
		func(_ context.Context, routines []*model.BackupRoutine) {
			require.Len(t, routines, 1)
			require.Equal(t, "changed", routines[0].Name)
		})

	applier := NewConfigApplier(
		NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil)),
		registry,
		nsValidator,
		cfg,
	)

	require.NoError(t, applier.ApplyNewConfig(t.Context()))
}

func noopNamespaceValidator(ctrl *gomock.Controller) aerospike.NamespaceValidator {
	nsValidator := aerospike.NewMockNamespaceValidator(ctrl)
	nsValidator.EXPECT().Validate(gomock.Any(), gomock.Any()).AnyTimes()

	return nsValidator
}
