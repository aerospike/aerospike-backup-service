package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	numGoroutines = 100
	byteWritten   = 100
)

// mockRestoreHandler implements the restoreexecutor.RestoreHandler for testing.
type mockRestoreHandler struct{}

func (m *mockRestoreHandler) GetStats() *models.RestoreStats {
	stats := models.NewRestoreStats()
	stats.ReadRecords.Add(1)
	stats.BytesWritten.Add(byteWritten)
	return stats
}

func (m *mockRestoreHandler) Wait(_ context.Context) error {
	return nil
}

func (m *mockRestoreHandler) GetMetrics() *models.Metrics {
	return nil
}

var _ restoreexecutor.RestoreHandler = (*mockRestoreHandler)(nil)

func TestRestoreJobsHolder_ConcurrentModification(t *testing.T) {
	t.Run("concurrently modify job and succeed", func(t *testing.T) {
		holder := NewRestoreJobsHolder()
		jobID := holder.newJob("test-label", func() {})

		recordsPerGoroutine := uint64(10)
		var wg sync.WaitGroup
		wg.Add(numGoroutines * 2)

		for range numGoroutines {
			// Concurrently add handlers
			go func() {
				defer wg.Done()
				holder.addHandler(jobID, &mockRestoreHandler{})
			}()

			// Concurrently add total records
			go func() {
				defer wg.Done()
				holder.addTotalRecords(jobID, recordsPerGoroutine)
			}()
		}

		wg.Wait()

		job, err := holder.getJob(jobID)
		require.NoError(t, err)
		status := job.buildStatus()
		assert.NotNil(t, status)

		// Assert against the fields of the returned status object
		assert.Equal(t, model.RestoreRunning, status.Status) // Job is still running until finishJob is called
		assert.NotNil(t, status.CurrentRestore)
		assert.Equal(t, uint64(numGoroutines)*recordsPerGoroutine, status.CurrentRestore.TotalRecords,
			"TotalRecords should be %d", uint64(numGoroutines)*recordsPerGoroutine)
		assert.Equal(t, uint64(numGoroutines), status.CurrentRestore.DoneRecords,
			"DoneRecords should be %d", uint64(numGoroutines)) // Each handler adds 1 record
		assert.NotNil(t, status.Counters)
		assert.Equal(t, uint64(numGoroutines), status.Counters.GetReadRecords(),
			"Counters.Records should be %d", uint64(numGoroutines))
		assert.Equal(t, uint64(numGoroutines)*byteWritten, status.Counters.GetBytesWritten(),
			"Counters.Bytes should be %d", uint64(numGoroutines)*byteWritten)

		// finish job
		holder.finishJob(jobID, nil, slog.New(slog.DiscardHandler))
		job, err = holder.getJob(jobID)
		require.NoError(t, err)
		status = job.buildStatus()
		assert.NotNil(t, status)

		require.NoError(t, err)
		assert.NotNil(t, status)

		assert.Equal(t, model.RestoreDone, status.Status)
	})

	t.Run("job is canceled", func(t *testing.T) {
		holder := NewRestoreJobsHolder()
		jobID := holder.newJob("test-label", func() {})
		recordsPerGoroutine := uint64(10)

		var wg sync.WaitGroup
		wg.Add(numGoroutines*2 + 1)

		for range numGoroutines {
			go func() {
				defer wg.Done()
				holder.addHandler(jobID, &mockRestoreHandler{})
			}()
			go func() {
				defer wg.Done()
				holder.addTotalRecords(jobID, recordsPerGoroutine)
			}()
		}

		go func() {
			defer wg.Done()
			// finish job with cancellation
			holder.finishJob(jobID, context.Canceled, slog.New(slog.DiscardHandler))
		}()

		wg.Wait()

		job, err := holder.getJob(jobID)
		require.NoError(t, err)
		status := job.buildStatus()
		assert.NotNil(t, status)

		require.NoError(t, err)
		assert.NotNil(t, status)

		assert.Equal(t, model.RestoreCanceled, status.Status)
		job, err = holder.getJob(jobID)
		require.NoError(t, err)
		require.ErrorIs(t, job.err, context.Canceled)
	})

	t.Run("job is failed", func(t *testing.T) {
		holder := NewRestoreJobsHolder()
		jobID := holder.newJob("test-label", func() {})
		failErr := errors.New("something went wrong")
		recordsPerGoroutine := uint64(10)

		var wg sync.WaitGroup
		wg.Add(numGoroutines*2 + 1)

		for range numGoroutines {
			go func() {
				defer wg.Done()
				holder.addHandler(jobID, &mockRestoreHandler{})
			}()
			go func() {
				defer wg.Done()
				holder.addTotalRecords(jobID, recordsPerGoroutine)
			}()
		}

		go func() {
			defer wg.Done()
			// finish job with failure
			holder.finishJob(jobID, failErr, slog.New(slog.DiscardHandler))
		}()

		wg.Wait()

		job, err := holder.getJob(jobID)
		require.NoError(t, err)
		status := job.buildStatus()
		assert.NotNil(t, status)

		require.NoError(t, err)
		assert.NotNil(t, status)

		assert.Equal(t, model.RestoreFailed, status.Status)
		job, err = holder.getJob(jobID)
		require.NoError(t, err)
		require.ErrorIs(t, job.err, failErr)
	})

	t.Run("job failed due to restore pre-requisites", func(t *testing.T) {
		holder := NewRestoreJobsHolder()
		jobID := holder.newJob("test-label", func() {})
		failErr := errors.Join(
			ErrRestorePrerequisitesFailed,
			errors.New("destination cluster does not have required namespace: ns1"),
		)

		holder.finishJob(jobID, failErr, slog.New(slog.DiscardHandler))

		job, err := holder.getJob(jobID)
		require.NoError(t, err)

		status := job.buildStatus()
		assert.NotNil(t, status)
		assert.Equal(t, model.RestoreFailed, status.Status)
		require.ErrorIs(t, job.err, ErrRestorePrerequisitesFailed)
	})
}

func TestRestoreJobsHolder_StatusCounts(t *testing.T) {
	holder := NewRestoreJobsHolder()
	jobRunning := holder.newJob("running", func() {})
	jobDone := holder.newJob("done", func() {})
	jobCanceled := holder.newJob("canceled", func() {})
	jobFailed := holder.newJob("failed", func() {})

	holder.finishJob(jobDone, nil, slog.New(slog.DiscardHandler))
	holder.finishJob(jobCanceled, context.Canceled, slog.New(slog.DiscardHandler))
	holder.finishJob(jobFailed, errors.New("failed"), slog.New(slog.DiscardHandler))

	counts := holder.StatusCounts()
	assert.Equal(t, 1, counts[model.RestoreRunning])
	assert.Equal(t, 1, counts[model.RestoreDone])
	assert.Equal(t, 1, counts[model.RestoreCanceled])
	assert.Equal(t, 1, counts[model.RestoreFailed])

	holder.finishJob(jobRunning, nil, slog.New(slog.DiscardHandler))
	counts = holder.StatusCounts()
	assert.Zero(t, counts[model.RestoreRunning])
	assert.Equal(t, 2, counts[model.RestoreDone])
	assert.Equal(t, 1, counts[model.RestoreCanceled])
	assert.Equal(t, 1, counts[model.RestoreFailed])
}
