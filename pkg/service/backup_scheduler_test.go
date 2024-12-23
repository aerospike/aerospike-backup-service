package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/reugn/go-quartz/quartz"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNeedToRunFullBackupNow(t *testing.T) {
	tests := []struct {
		name        string
		lastFullRun time.Time
		trigger     *quartz.CronTrigger
		expected    bool
	}{
		{
			name:        "NoPreviousBackup",
			lastFullRun: time.Time{},
			trigger:     newTrigger(""),
			expected:    true,
		},
		{
			name:        "DueForBackup",
			lastFullRun: time.Now().Add(-25 * time.Hour),
			trigger:     newTrigger("@daily"),
			expected:    true,
		},
		{
			name:        "NoNeedForBackup",
			lastFullRun: time.Now().Add(-10 * time.Second),
			trigger:     newTrigger("@daily"),
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needToRunFullBackupNow(tt.lastFullRun, tt.trigger); got != tt.expected {
				t.Errorf("needToRunFullBackupNow() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func newTrigger(expression string) *quartz.CronTrigger {
	trigger, _ := quartz.NewCronTrigger(expression)
	return trigger
}

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

	config := &model.Config{
		BackupRoutines: map[string]*model.BackupRoutine{
			"routine1": {Disabled: true},
			"routine2": {Disabled: false, IntervalCron: "@daily"},
		},
	}

	handlers := BackupHandlerHolder{
		"routine1": &BackupRoutineHandler{},
		"routine2": &BackupRoutineHandler{lastRun: lastBackupRun{
			full: time.Now(),
		}},
	}

	err := scheduleRoutines(mockScheduler, config, handlers)

	require.NoError(t, err)
	mockScheduler.AssertNumberOfCalls(t, "ScheduleJob", 1)
}
