package service

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/reugn/go-quartz/quartz"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockScheduler implements quartz.Scheduler for testing.
type MockScheduler struct {
	mock.Mock
}

func (m *MockScheduler) ScheduleJob(detail *quartz.JobDetail, trigger quartz.Trigger) error {
	args := m.Called(detail, trigger)
	return args.Error(0)
}

func TestScheduleRoutines(t *testing.T) {
	tests := []struct {
		name              string
		routines          map[string]*model.BackupRoutine
		expectedCalls     int
		expectedJobsAdded int
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
			expectedCalls:     2, // One for full backup, one for incremental
			expectedJobsAdded: 2, // Full and incremental jobs added
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
			expectedCalls:     0, // No calls expected for disabled routine
			expectedJobsAdded: 0,
		},
		{
			name: "full backup only",
			routines: map[string]*model.BackupRoutine{
				"full-only": {
					Name:         "full-only",
					IntervalCron: "0 0 * * * *",
				},
			},
			expectedCalls:     1, // One call for full backup only
			expectedJobsAdded: 2, // Full job plus ad-hoc incremental job added
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobStore.ReplaceContent(map[string]*quartz.JobDetail{})

			scheduler := new(MockScheduler)
			scheduler.On("ScheduleJob", mock.Anything, mock.Anything).Return(nil)

			routines := make([]*model.BackupRoutine, 0, len(tt.routines))
			for _, routine := range tt.routines {
				routines = append(routines, routine)
			}
			err := scheduleRoutines(scheduler, routines, &BackupComponents{}, nil)

			require.NoError(t, err)
			scheduler.AssertNumberOfCalls(t, "ScheduleJob", tt.expectedCalls)
			require.Equal(t, tt.expectedJobsAdded, jobStore.Size())
		})
	}
}
