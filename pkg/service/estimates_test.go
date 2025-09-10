package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestNewRunningJob(t *testing.T) {
	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	finishTime := startTime.Add(time.Hour)

	t.Run("zero total returns minimal job", func(t *testing.T) {
		result := NewRunningJob(startTime, &finishTime, 0, 0)

		assert.Equal(t, startTime, result.StartTime)
		assert.Equal(t, &finishTime, result.FinishTime)
		assert.Zero(t, result.DoneRecords)
		assert.Zero(t, result.TotalRecords)
		assert.Nil(t, result.EstimatedEndTime)
		assert.Zero(t, result.PercentageDone)
	})

	t.Run("zero progress", func(t *testing.T) {
		result := NewRunningJob(startTime, nil, 0, 100)

		assert.Zero(t, result.DoneRecords)
		assert.Zero(t, result.PercentageDone)
		assert.Equal(t, uint64(100), result.TotalRecords)
		assert.Nil(t, result.EstimatedEndTime) // too early to estimate
	})

	t.Run("50 percent completion", func(t *testing.T) {
		result := NewRunningJob(startTime, nil, 50, 100)

		assert.Equal(t, uint64(50), result.DoneRecords)
		assert.Equal(t, uint64(100), result.TotalRecords)
		assert.Equal(t, uint(50), result.PercentageDone)
		assert.True(t, result.EstimatedEndTime.After(startTime)) // should be in the future
	})

	t.Run("completed job", func(t *testing.T) {
		result := NewRunningJob(startTime, &finishTime, 100, 100)

		assert.Equal(t, uint64(100), result.DoneRecords)
		assert.Equal(t, uint64(100), result.TotalRecords)
		assert.Equal(t, uint(100), result.PercentageDone)
		assert.Equal(t, &finishTime, result.FinishTime)
	})
}

func TestDoneRestoreJobStatus(t *testing.T) {
	job := &restoreJob{
		status: model.JobStatusDone,
	}

	status := RestoreJobStatus(job)

	assert.Nil(t, status.Error)
	assert.Nil(t, status.CurrentRestore)
	assert.Equal(t, status.Status, model.JobStatusDone)
}
