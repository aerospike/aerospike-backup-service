package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/quartz"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockScheduler implements JobScheduler for testing.
type MockScheduler struct {
	mock.Mock
}

func (m *MockScheduler) ScheduleJob(detail *quartz.JobDetail, trigger quartz.Trigger) error {
	args := m.Called(detail, trigger)
	return args.Error(0)
}

func (m *MockScheduler) DeleteJob(key *quartz.JobKey) error {
	args := m.Called(key)
	return args.Error(0)
}

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
				},
			},
			expectedCalls: 1, // One call for full backup only
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduler := new(MockScheduler)
			scheduler.On("ScheduleJob", mock.Anything, mock.Anything).Return(nil)

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
			scheduler.AssertNumberOfCalls(t, "ScheduleJob", tt.expectedCalls)
		})
	}
}

func TestBackupScheduler_DeleteJob(t *testing.T) {
	key := jobKey("routine-1", model.BackupTypeFull)
	scheduler := new(MockScheduler)
	scheduler.On("DeleteJob", key).Return(nil)

	bs := NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil))
	require.NoError(t, bs.DeleteJob(key))
	scheduler.AssertExpectations(t)
}

func TestBackupScheduler_TriggerAdHocFullBackup(t *testing.T) {
	routine := &model.BackupRoutine{Name: "routine-1"}
	scheduler := new(MockScheduler)
	scheduler.On(
		"ScheduleJob",
		mock.MatchedBy(adHocJobMatcher("routine-1", model.BackupTypeFull)),
		mock.Anything,
	).Return(nil)

	bs := NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil))
	require.NoError(t, bs.TriggerAdHocFullBackup(routine, time.Second))
	scheduler.AssertExpectations(t)
}

func TestBackupScheduler_TriggerAdHocIncrementalBackup(t *testing.T) {
	routine := &model.BackupRoutine{Name: "routine-1"}
	scheduler := new(MockScheduler)
	scheduler.On(
		"ScheduleJob",
		mock.MatchedBy(adHocJobMatcher("routine-1", model.BackupTypeIncremental)),
		mock.Anything,
	).Return(nil)

	bs := NewBackupScheduler(scheduler, NewBackupOrchestrator(nil, nil, nil, nil, nil))
	require.NoError(t, bs.TriggerAdHocIncrementalBackup(routine, time.Second))
	scheduler.AssertExpectations(t)
}

func TestBackupScheduler_TriggerAdHocBackup_EnforcesMinimumDelay(t *testing.T) {
	routine := &model.BackupRoutine{Name: "routine-1"}
	scheduler := new(MockScheduler)
	scheduler.On(
		"ScheduleJob",
		mock.MatchedBy(adHocJobMatcher("routine-1", model.BackupTypeFull)),
		mock.MatchedBy(func(trigger quartz.Trigger) bool {
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
	scheduler.AssertExpectations(t)
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
