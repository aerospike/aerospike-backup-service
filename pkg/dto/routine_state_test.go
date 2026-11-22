package dto

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRoutineStateFromModel(t *testing.T) {
	t.Parallel()

	lastFull := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	lastIncr := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	nextFull := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)
	nextIncr := time.Date(2025, 1, 4, 0, 0, 0, 0, time.UTC)

	m := model.RoutineState{
		Full: &model.RunningJob{
			TotalRecords: 100,
			DoneRecords:  50,
			StartTime:    lastFull,
			Progress:     0.5,
		},
		LastRunTime: model.NewBackupTime(lastFull, lastIncr),
		NextRunTime: model.NewBackupTime(nextFull, nextIncr),
	}

	state := NewRoutineStateFromModel(m)
	require.NotNil(t, state.Full)
	assert.Nil(t, state.Incremental)
	assert.Equal(t, uint64(100), state.Full.TotalRecords)
	assert.Equal(t, uint(50), state.Full.PercentageDone)
	require.NotNil(t, state.LastFull)
	assert.Equal(t, lastFull, *state.LastFull)
	require.NotNil(t, state.LastIncremental)
	assert.Equal(t, lastIncr, *state.LastIncremental)
	require.NotNil(t, state.NextFull)
	assert.Equal(t, nextFull, *state.NextFull)
	require.NotNil(t, state.NextIncremental)
	assert.Equal(t, nextIncr, *state.NextIncremental)
}

func TestNewRoutineStateFromModel_ServerJobs(t *testing.T) {
	t.Parallel()

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	finish := start.Add(time.Minute)
	job := model.ServerBackupJob{
		JobID: "job", Namespace: "test", State: "ABORTED", ErrorReason: "node left", AbortOrigin: "cluster",
		StartTime: &start, FinishTime: &finish, RecordsBackedUp: 10, BytesBackedUp: 100, RecordsReadFailed: 1,
	}

	state := NewRoutineStateFromModel(model.RoutineState{
		LastRunTime:  model.NewNoBackupTime(),
		NextRunTime:  model.NewNoBackupTime(),
		LastFullJobs: []model.ServerBackupJob{job},
	})

	assert.Equal(t, []ServerBackupJob{{
		JobID: "job", Namespace: "test", State: "ABORTED", ErrorReason: "node left", AbortOrigin: "cluster",
		StartTime: &start, FinishTime: &finish, RecordsBackedUp: 10, BytesBackedUp: 100, RecordsReadFailed: 1,
	}}, state.LastFullJobs)
	assert.Empty(t, state.LastIncrementalJobs)
}

// TestNewRunningJobFromModel_ServerJobs checks that a running server-side backup lists its server
// jobs, and that a scan backup lists none.
func TestNewRunningJobFromModel_ServerJobs(t *testing.T) {
	t.Parallel()

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	running := NewRunningJobFromModel(&model.RunningJob{
		StartTime: start,
		ServerJobs: []model.ServerBackupJob{
			{JobID: "job-bar", Namespace: "bar", State: "COMPLETE", RecordsBackedUp: 10},
			{JobID: "job-baz", Namespace: "baz", State: "BASE_SCAN_ACTIVE", StartTime: &start, RecordsBackedUp: 3},
		},
	})

	assert.Equal(t, []ServerBackupJob{
		{JobID: "job-bar", Namespace: "bar", State: "COMPLETE", RecordsBackedUp: 10},
		{JobID: "job-baz", Namespace: "baz", State: "BASE_SCAN_ACTIVE", StartTime: &start, RecordsBackedUp: 3},
	}, running.ServerJobs)
	assert.Empty(t, NewRunningJobFromModel(&model.RunningJob{StartTime: start}).ServerJobs)
}

func TestNewRunningJobFromModel_Nil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, NewRunningJobFromModel(nil))
}

func TestNewRunningJobFromModel_Finished(t *testing.T) {
	t.Parallel()

	start := time.Now().Add(-10 * time.Second)
	finish := time.Now()

	job := NewRunningJobFromModel(&model.RunningJob{
		TotalRecords: 10,
		DoneRecords:  10,
		StartTime:    start,
		FinishTime:   &finish,
		Progress:     1,
	})

	require.NotNil(t, job)
	assert.Equal(t, uint(100), job.PercentageDone)
	assert.InDelta(t, uint(finish.Sub(start).Seconds()), job.Duration, 1)
}

func TestNewRunningJobFromModel_StillRunning(t *testing.T) {
	t.Parallel()

	start := time.Now().Add(-5 * time.Second)

	job := NewRunningJobFromModel(&model.RunningJob{
		StartTime: start,
		Progress:  0.25,
	})

	require.NotNil(t, job)
	assert.Nil(t, job.FinishTime)
	assert.GreaterOrEqual(t, job.Duration, uint(5))
}
