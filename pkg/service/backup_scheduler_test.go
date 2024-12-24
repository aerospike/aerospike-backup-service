package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
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

	config := &model.Config{
		BackupRoutines: map[string]*model.BackupRoutine{
			"routine1": {Disabled: true},
			"routine2": {Disabled: false, IntervalCron: "@daily"},
		},
	}

	handlers := BackupHandlerHolder{
		"routine1": &BackupRoutineHandler{},
		"routine2": &BackupRoutineHandler{lastRun: model.LastBackupRun{
			Full: util.Ptr(time.Now()),
		}},
	}

	err := scheduleRoutines(mockScheduler, config, handlers)

	require.NoError(t, err)
	mockScheduler.AssertNumberOfCalls(t, "ScheduleJob", 1)
}
