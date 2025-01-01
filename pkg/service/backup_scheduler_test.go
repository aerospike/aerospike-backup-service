package service

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
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

// TestDisabledRoutine verifies that disabled routines are skipped in scheduleRoutines.
func TestDisabledRoutine(t *testing.T) {
	mockScheduler := new(MockScheduler)
	mockScheduler.On("ScheduleJob", mock.Anything, mock.Anything).Return(nil)

	config := model.NewConfig()
	_ = config.AddRoutine("routine1", &model.BackupRoutine{Disabled: true})
	_ = config.AddRoutine("routine2", &model.BackupRoutine{IntervalCron: "@daily"})

	handlers := BackupHandlerHolder{
		"routine1": &BackupRoutineHandler{},
		"routine2": &BackupRoutineHandler{lastRun: model.NewLastBackupRun(util.Ptr(time.Now()), nil)},
	}

	err := scheduleRoutines(mockScheduler, config.Routines(), handlers)

	require.NoError(t, err)
	mockScheduler.AssertNumberOfCalls(t, "ScheduleJob", 1)
}

// MockBackupRunner is a mock implementation of backupRunner interface
type MockBackupRunner struct {
	mock.Mock
}

func (m *MockBackupRunner) runFullBackup(ctx context.Context, t time.Time) {
	m.Called(ctx, t)
}

func (m *MockBackupRunner) runIncrementalBackup(ctx context.Context, t time.Time) {
	m.Called(ctx, t)
}

func (m *MockBackupRunner) Cancel() {
	m.Called()
}

func (m *MockBackupRunner) CurrentStat() *model.CurrentBackups {
	args := m.Called()
	return args.Get(0).(*model.CurrentBackups)
}

func TestScheduleRoutines(t *testing.T) {
	holder := BackupHandlerHolder{
		"routine":          &MockBackupRunner{},
		"disabled-routine": &MockBackupRunner{},
		"full-only":        &MockBackupRunner{},
	}
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
					IntervalCron:     "0 0 * * * *",
					IncrIntervalCron: "0 */6 * * * *",
				},
			},
			expectedCalls:     2, // One for full backup, one for incremental
			expectedJobsAdded: 1, // Only full backup job added
		},
		{
			name: "skip disabled routine",
			routines: map[string]*model.BackupRoutine{
				"disabled-routine": {
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
					IntervalCron: "0 0 * * * *",
				},
			},
			expectedCalls:     1, // One call for full backup only
			expectedJobsAdded: 1, // Only full backup job added
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduler := new(MockScheduler)
			scheduler.On("ScheduleJob", mock.Anything, mock.Anything).Return(nil)

			err := scheduleRoutines(scheduler, tt.routines, holder)

			require.NoError(t, err)
			scheduler.AssertNumberOfCalls(t, "ScheduleJob", tt.expectedCalls)
			require.Equal(t, len(jobStore.jobs), tt.expectedJobsAdded)
		})
	}
}
