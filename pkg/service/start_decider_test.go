package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/timeutil"
	"github.com/stretchr/testify/assert"
)

func TestCanStartIncrementalBackup(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	lastFullBackupTime := now.Add(-25 * time.Hour)

	tests := []struct {
		name           string
		routineState   model.RoutineState
		concurrent     bool
		now            time.Time
		expectedToSkip bool
	}{
		{
			name: "don't skip usually",
			routineState: model.RoutineState{
				LastRunTime: model.NewFullBackupTime(lastFullBackupTime),
			},
			now:            now.Add(1 * time.Hour),
			expectedToSkip: false,
		},
		{
			name:           "skip when no full backup",
			routineState:   model.RoutineState{LastRunTime: model.NewNoBackupTime()},
			expectedToSkip: true,
		},
		{
			name: "skip when full backup in progress",
			routineState: model.RoutineState{
				LastRunTime: model.NewFullBackupTime(now.Add(-24 * time.Hour)),
				Full:        &model.RunningJob{StartTime: now},
			},
			expectedToSkip: true,
		},
		{
			name: "don't skip when full backup in progress and concurrent allowed",
			routineState: model.RoutineState{
				LastRunTime: model.NewFullBackupTime(now.Add(-24 * time.Hour)),
				Full:        &model.RunningJob{StartTime: now},
			},
			concurrent:     true,
			expectedToSkip: false,
		},
		{
			name: "skip when full backup is scheduled at same time",
			routineState: model.RoutineState{
				LastRunTime: model.NewFullBackupTime(lastFullBackupTime),
			},
			now:            now,
			expectedToSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routine := testRoutine()
			if tt.concurrent {
				routine.BackupPolicy.ConcurrentIncremental = ptr.Of(true)
			}

			facts := StartFacts{
				FullRunningNow:        tt.routineState.Full != nil,
				IncrementalRunningNow: tt.routineState.Incremental != nil,
				HasCompletedFull:      !tt.routineState.LastRunTime.NoFullBackup(),
				FullScheduledNow:      timeutil.IsCronFireTime(routine.IntervalCron, tt.now),
			}

			policy := NewStartDecider()
			err := policy.CanStart(jobTypeIncremental, routine, facts)
			assert.Equal(t, tt.expectedToSkip, err != nil)
		})
	}
}

func TestCanStartFullBackup(t *testing.T) {
	tests := []struct {
		name        string
		facts       StartFacts
		expectedErr error
	}{
		{
			name: "allow full backup when no full is running",
			facts: StartFacts{
				FullRunningNow: false,
			},
			expectedErr: nil,
		},
		{
			name: "deny full backup when another full is running",
			facts: StartFacts{
				FullRunningNow: true,
			},
			expectedErr: errFullAlreadyRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routine := testRoutine()
			policy := NewStartDecider()

			err := policy.CanStart(jobTypeFull, routine, tt.facts)
			if tt.expectedErr == nil {
				assert.NoError(t, err)
				return
			}

			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
