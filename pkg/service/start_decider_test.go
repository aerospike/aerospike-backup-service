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
		intervalCron   string
		now            time.Time
		expectedToSkip bool
	}{
		{
			name: "don't skip usually",
			routineState: model.RoutineState{
				LastRunTime: model.NewFullBackupTime(lastFullBackupTime),
			},
			intervalCron:   "@daily",
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
			intervalCron:   "@daily",
			now:            now,
			expectedToSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routine := testRoutine()
			routine.IntervalCron = tt.intervalCron
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
