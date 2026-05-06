package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRunningJob(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	finishTime := startTime.Add(time.Hour)
	jobRunning := model.RestoreRunning

	t.Run("zero total returns minimal job", func(t *testing.T) {
		result := NewRestoreRunningJob(startTime, &finishTime, 0, 0, nil, jobRunning)

		assert.Equal(t, startTime, result.StartTime)
		assert.Equal(t, &finishTime, result.FinishTime)
		assert.Zero(t, result.DoneRecords)
		assert.Zero(t, result.TotalRecords)
		assert.Nil(t, result.EstimatedEndTime)
		assert.Zero(t, result.Progress)
	})

	t.Run("zero progress", func(t *testing.T) {
		result := NewRestoreRunningJob(startTime, nil, 0, 100, nil, jobRunning)

		assert.Zero(t, result.DoneRecords)
		assert.Zero(t, result.Progress)
		assert.Equal(t, uint64(100), result.TotalRecords)
		assert.Nil(t, result.EstimatedEndTime) // too early to estimate
	})

	t.Run("50 percent completion", func(t *testing.T) {
		result := NewRestoreRunningJob(startTime, nil, 50, 100, nil, jobRunning)

		assert.Equal(t, uint64(50), result.DoneRecords)
		assert.Equal(t, uint64(100), result.TotalRecords)
		assert.InDelta(t, 0.5, result.Progress, 0.01)
		assert.True(t, result.EstimatedEndTime.After(startTime)) // should be in the future
	})

	t.Run("completed job", func(t *testing.T) {
		result := NewRestoreRunningJob(startTime, &finishTime, 100, 100, nil, jobRunning)

		assert.Equal(t, uint64(100), result.DoneRecords)
		assert.Equal(t, uint64(100), result.TotalRecords)
		assert.InDelta(t, 0.99, result.Progress, 0.01)
		assert.Equal(t, &finishTime, result.FinishTime)
	})

	t.Run("exceed 100%", func(t *testing.T) {
		result := NewRestoreRunningJob(startTime, nil, 110, 100, nil, jobRunning)

		assert.Equal(t, uint64(110), result.DoneRecords)
		assert.Equal(t, uint64(100), result.TotalRecords)
		assert.InDelta(t, 0.99, result.Progress, 0.01)
	})
}

func TestDoneRestoreJobStatus(t *testing.T) {
	job := &restoreJob{
		status:       model.RestoreSuccess,
		totalRecords: 50,
	}

	status := job.buildStatus()

	require.NoError(t, status.Error)
	assert.NotNil(t, status.CurrentRestore)
	assert.InDelta(t, 1.0, status.CurrentRestore.Progress, 0.01)
	assert.Equal(t, model.RestoreSuccess, status.Status)
}
