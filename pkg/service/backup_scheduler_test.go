package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/quartz"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestScheduleRoutines(t *testing.T) {
	tests := []struct {
		name          string
		routines      map[string]*model.BackupRoutine
		expectedCalls int
	}{
		{
			name: "successful scheduling of full and incremental backups",
			routines: map[string]*model.BackupRoutine{
				"routine": {
					Name:             "routine",
					IntervalCron:     "0 0 * * * *",
					IncrIntervalCron: "0 */6 * * * *",
					Timezone:         time.UTC,
				},
			},
			expectedCalls: 2, // One for full backup, one for incremental
		},
		{
			name: "skip disabled routine",
			routines: map[string]*model.BackupRoutine{
				"disabled-routine": {
					Name:             "disabled-routine",
					IntervalCron:     "0 0 * * * *",
					IncrIntervalCron: "0 */6 * * * *",
					Disabled:         true,
				},
			},
			expectedCalls: 0, // No calls expected for disabled routine
		},
		{
			name: "full backup only",
			routines: map[string]*model.BackupRoutine{
				"full-only": {
					Name:         "full-only",
					IntervalCron: "0 0 * * * *",
					Timezone:     time.UTC,
				},
			},
			expectedCalls: 1, // One call for full backup only
		},
		{
			name: "named timezone still schedules",
			routines: map[string]*model.BackupRoutine{
				"ny-routine": {
					Name:         "ny-routine",
					IntervalCron: "0 0 2 * * *",
					Timezone:     time.FixedZone("America/New_York", -4*60*60),
				},
			},
			expectedCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			scheduler := NewMockJobScheduler(ctrl)
			scheduler.EXPECT().
				ScheduleJob(gomock.Any(), gomock.Any()).
				Return(nil).
				Times(tt.expectedCalls)

			routines := make([]*model.BackupRoutine, 0, len(tt.routines))
			for _, routine := range tt.routines {
				routines = append(routines, routine)
			}
			bs := NewBackupScheduler(scheduler, NewBackupOrchestrator(
				nil,
				nil,
				nil,
				nil,
				nil,
			))
			err := bs.ScheduleRoutines(routines)

			require.NoError(t, err)
		})
	}
}

func TestScheduleRoutines_UsesRoutineTimezone(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	scheduler := NewMockJobScheduler(ctrl)
	scheduler.EXPECT().ScheduleJob(
		gomock.Cond(func(detail *quartz.JobDetail) bool {
			job, ok := detail.Job().(*backupJob)
			return ok && job.routine.Timezone == location
		}),
		gomock.Cond(func(trigger quartz.Trigger) bool {
			after := time.Date(2025, 7, 20, 0, 0, 0, 0, time.UTC)
			next, triggerErr := trigger.NextFireTime(after.UnixNano())
			if triggerErr != nil {
				return false
			}

			return time.Unix(0, next).Equal(time.Date(2025, 7, 20, 4, 0, 0, 0, time.UTC))
		}),
	).Return(nil)

	backupScheduler := NewBackupScheduler(
		scheduler,
		NewBackupOrchestrator(nil, nil, nil, nil, nil),
	)
	require.NoError(t, backupScheduler.ScheduleRoutines([]*model.BackupRoutine{{
		Name:         "ny-timezone",
		IntervalCron: "@daily",
		Timezone:     location,
	}}))
}

func TestBackupScheduler_DeleteJob(t *testing.T) {
	ctrl := gomock.NewController(t)
	key := jobKey("routine-1", model.BackupTypeFull)
	scheduler := NewMockJobScheduler(ctrl)
	scheduler.EXPECT().DeleteJob(key).Return(nil)

	bs := NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil))
	require.NoError(t, bs.DeleteJob(key))
}

func TestBackupScheduler_TriggerAdHocFullBackup(t *testing.T) {
	ctrl := gomock.NewController(t)
	routine := &model.BackupRoutine{Name: "routine-1"}
	scheduler := NewMockJobScheduler(ctrl)
	scheduler.EXPECT().ScheduleJob(
		gomock.Cond(adHocJobMatcher("routine-1", model.BackupTypeFull)),
		gomock.Any(),
	).Return(nil)

	bs := NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil))
	require.NoError(t, bs.TriggerAdHocFullBackup(routine, time.Second))
}

func TestBackupScheduler_TriggerAdHocIncrementalBackup(t *testing.T) {
	ctrl := gomock.NewController(t)
	routine := &model.BackupRoutine{Name: "routine-1"}
	scheduler := NewMockJobScheduler(ctrl)
	scheduler.EXPECT().ScheduleJob(
		gomock.Cond(adHocJobMatcher("routine-1", model.BackupTypeIncremental)),
		gomock.Any(),
	).Return(nil)

	bs := NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil))
	require.NoError(t, bs.TriggerAdHocIncrementalBackup(routine, time.Second))
}

func TestBackupScheduler_TriggerAdHocBackup_EnforcesMinimumDelay(t *testing.T) {
	ctrl := gomock.NewController(t)
	routine := &model.BackupRoutine{Name: "routine-1"}
	scheduler := NewMockJobScheduler(ctrl)
	scheduler.EXPECT().ScheduleJob(
		gomock.Cond(adHocJobMatcher("routine-1", model.BackupTypeFull)),
		gomock.Cond(func(trigger quartz.Trigger) bool {
			now := time.Now().UnixNano()
			next, err := trigger.NextFireTime(now)
			if err != nil {
				return false
			}
			delay := time.Duration(next - now)
			return delay >= minAdHocBackupDelay
		}),
	).Return(nil)

	bs := NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil))
	require.NoError(t, bs.TriggerAdHocFullBackup(routine, 0))
}

func adHocJobMatcher(routineName string, backupType model.BackupType) func(*quartz.JobDetail) bool {
	return func(detail *quartz.JobDetail) bool {
		job, ok := detail.Job().(*backupJob)
		if !ok {
			return false
		}
		return detail.JobKey().Group() == string(quartzGroupAdHoc) &&
			job.routine.Name == routineName &&
			job.backupType == backupType
	}
}
