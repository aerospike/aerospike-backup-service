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
		holder := newStartedJobsHolder(t)
		jobID := newJobID(holder, "test-label")

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

		assert.Equal(t, model.RestoreSuccess, status.Status)
	})

	t.Run("job is canceled", func(t *testing.T) {
		holder := newStartedJobsHolder(t)
		jobID := newJobID(holder, "test-label")
		recordsPerGoroutine := uint64(10)

		var wg sync.WaitGroup

		for range numGoroutines {
			wg.Go(func() {
				holder.addHandler(jobID, &mockRestoreHandler{})
			})
			wg.Go(func() {
				holder.addTotalRecords(jobID, recordsPerGoroutine)
			})
		}

		wg.Go(func() {
			// finish job with cancellation
			holder.finishJob(jobID, context.Canceled, slog.New(slog.DiscardHandler))
		})

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
		holder := newStartedJobsHolder(t)
		jobID := newJobID(holder, "test-label")
		failErr := errors.New("something went wrong")
		recordsPerGoroutine := uint64(10)

		var wg sync.WaitGroup

		for range numGoroutines {
			wg.Go(func() {
				holder.addHandler(jobID, &mockRestoreHandler{})
			})
			wg.Go(func() {
				holder.addTotalRecords(jobID, recordsPerGoroutine)
			})
		}

		wg.Go(func() {
			// finish job with failure
			holder.finishJob(jobID, failErr, slog.New(slog.DiscardHandler))
		})

		wg.Wait()

		job, err := holder.getJob(jobID)
		require.NoError(t, err)
		status := job.buildStatus()
		assert.NotNil(t, status)

		require.NoError(t, err)
		assert.NotNil(t, status)

		assert.Equal(t, model.RestoreFailure, status.Status)
		job, err = holder.getJob(jobID)
		require.NoError(t, err)
		require.ErrorIs(t, job.err, failErr)
	})

	t.Run("job failed due to restore pre-requisites", func(t *testing.T) {
		holder := newStartedJobsHolder(t)
		jobID := newJobID(holder, "test-label")
		failErr := errors.Join(
			ErrRestorePrerequisitesFailed,
			errors.New("destination cluster does not have required namespace: ns1"),
		)

		holder.finishJob(jobID, failErr, slog.New(slog.DiscardHandler))

		job, err := holder.getJob(jobID)
		require.NoError(t, err)

		status := job.buildStatus()
		assert.NotNil(t, status)
		assert.Equal(t, model.RestoreFailure, status.Status)
		require.ErrorIs(t, job.err, ErrRestorePrerequisitesFailed)
	})
}

func TestRestoreJobsHolder_JobsEndWithTheServiceContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	holder := NewRestoreJobsHolder()
	holder.Start(ctx)

	_, jobCtx := holder.newJob("running")
	require.NoError(t, jobCtx.Err())

	cancel()

	require.ErrorIs(t, jobCtx.Err(), context.Canceled, "a job must not outlive the service")
}

func TestRestoreJobsHolder_FinishReleasesJobContext(t *testing.T) {
	holder := newStartedJobsHolder(t)
	jobID, ctx := holder.newJob("done")

	holder.finishJob(jobID, nil, slog.New(slog.DiscardHandler))

	require.ErrorIs(t, ctx.Err(), context.Canceled, "a finished job must not keep its context alive")
	job, err := holder.getJob(jobID)
	require.NoError(t, err)
	assert.Equal(t, model.RestoreSuccess, job.getStatus(), "releasing the context does not change the outcome")
}

func TestRestoreJobsHolder_StatusCounts(t *testing.T) {
	holder := newStartedJobsHolder(t)
	jobRunning := newJobID(holder, "running")
	jobDone := newJobID(holder, "done")
	jobCanceled := newJobID(holder, "canceled")
	jobFailed := newJobID(holder, "failed")

	holder.finishJob(jobDone, nil, slog.New(slog.DiscardHandler))
	holder.finishJob(jobCanceled, context.Canceled, slog.New(slog.DiscardHandler))
	holder.finishJob(jobFailed, errors.New("failed"), slog.New(slog.DiscardHandler))

	counts := holder.StatusCounts()
	assert.Equal(t, 1, counts[model.RestoreRunning])
	assert.Equal(t, 1, counts[model.RestoreSuccess])
	assert.Equal(t, 1, counts[model.RestoreCanceled])
	assert.Equal(t, 1, counts[model.RestoreFailure])

	holder.finishJob(jobRunning, nil, slog.New(slog.DiscardHandler))
	counts = holder.StatusCounts()
	assert.Zero(t, counts[model.RestoreRunning])
	assert.Equal(t, 2, counts[model.RestoreSuccess])
	assert.Equal(t, 1, counts[model.RestoreCanceled])
	assert.Equal(t, 1, counts[model.RestoreFailure])
}

// newStartedJobsHolder returns a holder that accepts jobs for the duration of the test.
func newStartedJobsHolder(t *testing.T) *RestoreJobsHolder {
	t.Helper()

	holder := NewRestoreJobsHolder()
	holder.Start(t.Context())

	return holder
}

// newJobID starts a job and drops the context the caller under test does not need.
func newJobID(holder *RestoreJobsHolder, label string) model.RestoreJobID {
	id, _ := holder.newJob(label)

	return id
}
