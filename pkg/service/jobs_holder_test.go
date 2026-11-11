package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	numGoroutines = 100
	byteWritten   = 100
)

// mockRestoreHandler implements the RestoreHandler for testing.
type mockRestoreHandler struct{}

func (m *mockRestoreHandler) Status() model.RestoreRun {
	return model.RestoreRun{Counters: model.RestoreCounters{ReadRecords: 1, BytesWritten: byteWritten}}
}

func (m *mockRestoreHandler) Wait(_ context.Context) error {
	return nil
}

var _ RestoreHandler = (*mockRestoreHandler)(nil)

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
				holder.AddHandler(jobID, &mockRestoreHandler{})
			}()

			// Concurrently add total records
			go func() {
				defer wg.Done()
				holder.AddTotalRecords(jobID, recordsPerGoroutine)
			}()
		}

		wg.Wait()

		job, err := holder.getJob(jobID)
		require.NoError(t, err)
		status := job.buildStatus()
		assert.NotNil(t, status)

		// Assert against the fields of the returned status object
		assert.Equal(t, model.RestoreRunning, status.Status) // Job is still running until FinishJob is called
		assert.NotNil(t, status.CurrentRestore)
		assert.Equal(t, uint64(numGoroutines)*recordsPerGoroutine, status.CurrentRestore.TotalRecords,
			"TotalRecords should be %d", uint64(numGoroutines)*recordsPerGoroutine)
		assert.Equal(t, uint64(numGoroutines), status.CurrentRestore.DoneRecords,
			"DoneRecords should be %d", uint64(numGoroutines)) // Each handler adds 1 record
		assert.Equal(t, uint64(numGoroutines), status.Counters.ReadRecords,
			"Counters.Records should be %d", uint64(numGoroutines))
		assert.Equal(t, uint64(numGoroutines)*byteWritten, status.Counters.BytesWritten,
			"Counters.Bytes should be %d", uint64(numGoroutines)*byteWritten)

		// finish job
		holder.FinishJob(jobID, nil, slog.New(slog.DiscardHandler))
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
				holder.AddHandler(jobID, &mockRestoreHandler{})
			})
			wg.Go(func() {
				holder.AddTotalRecords(jobID, recordsPerGoroutine)
			})
		}

		wg.Go(func() {
			// finish job with cancellation
			holder.FinishJob(jobID, context.Canceled, slog.New(slog.DiscardHandler))
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
				holder.AddHandler(jobID, &mockRestoreHandler{})
			})
			wg.Go(func() {
				holder.AddTotalRecords(jobID, recordsPerGoroutine)
			})
		}

		wg.Go(func() {
			// finish job with failure
			holder.FinishJob(jobID, failErr, slog.New(slog.DiscardHandler))
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

		holder.FinishJob(jobID, failErr, slog.New(slog.DiscardHandler))

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

	_, jobCtx := holder.NewJob("running")
	require.NoError(t, jobCtx.Err())

	cancel()

	require.ErrorIs(t, jobCtx.Err(), context.Canceled, "a job must not outlive the service")
}

func TestRestoreJobsHolder_FinishReleasesJobContext(t *testing.T) {
	holder := newStartedJobsHolder(t)
	jobID, ctx := holder.NewJob("done")

	holder.FinishJob(jobID, nil, slog.New(slog.DiscardHandler))

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

	holder.FinishJob(jobDone, nil, slog.New(slog.DiscardHandler))
	holder.FinishJob(jobCanceled, context.Canceled, slog.New(slog.DiscardHandler))
	holder.FinishJob(jobFailed, errors.New("failed"), slog.New(slog.DiscardHandler))

	counts := holder.StatusCounts()
	assert.Equal(t, 1, counts[model.RestoreRunning])
	assert.Equal(t, 1, counts[model.RestoreSuccess])
	assert.Equal(t, 1, counts[model.RestoreCanceled])
	assert.Equal(t, 1, counts[model.RestoreFailure])

	holder.FinishJob(jobRunning, nil, slog.New(slog.DiscardHandler))
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
	id, _ := holder.NewJob(label)

	return id
}

func TestRestoreJobsHolder_GetFilteredJobs(t *testing.T) {
	holder := NewRestoreJobsHolder()

	older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	holder.Store(model.RestoreJobID(1), &restoreJob{started: older, status: model.RestoreSuccess})
	holder.Store(model.RestoreJobID(2), &restoreJob{started: newer, status: model.RestoreFailure})
	holder.Store(model.RestoreJobID(3), &restoreJob{started: newer, status: model.RestoreRunning})

	t.Run("no filters returns all jobs", func(t *testing.T) {
		results := holder.GetFilteredJobs(model.TimeBounds{}, model.StatusFilter{})
		assert.Len(t, results, 3)
	})

	t.Run("time bounds filter excludes jobs outside the window", func(t *testing.T) {
		from := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
		results := holder.GetFilteredJobs(model.TimeBounds{FromTime: &from}, model.StatusFilter{})
		assert.Len(t, results, 2)
		assert.NotContains(t, results, model.RestoreJobID(1))
	})

	t.Run("status filter includes only matching statuses", func(t *testing.T) {
		statusFilter := model.NewStatusFilter([]model.RestoreState{model.RestoreSuccess}, false)
		results := holder.GetFilteredJobs(model.TimeBounds{}, statusFilter)
		assert.Len(t, results, 1)
		assert.Contains(t, results, model.RestoreJobID(1))
	})

	t.Run("status filter in exclude mode omits matching statuses", func(t *testing.T) {
		statusFilter := model.NewStatusFilter([]model.RestoreState{model.RestoreRunning}, true)
		results := holder.GetFilteredJobs(model.TimeBounds{}, statusFilter)
		assert.Len(t, results, 2)
		assert.NotContains(t, results, model.RestoreJobID(3))
	})

	t.Run("combined time bounds and status filter", func(t *testing.T) {
		from := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
		statusFilter := model.NewStatusFilter([]model.RestoreState{model.RestoreFailure}, false)
		results := holder.GetFilteredJobs(model.TimeBounds{FromTime: &from}, statusFilter)
		assert.Len(t, results, 1)
		assert.Contains(t, results, model.RestoreJobID(2))
	})
}

// A job id the holder does not know is reported as a JobNotFoundError, which the HTTP layer
// answers with 404.
func TestRestoreJobsHolder_UnknownJob(t *testing.T) {
	t.Parallel()
	holder := newStartedJobsHolder(t)

	var notFound *JobNotFoundError

	_, err := holder.JobStatus(42)
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, model.RestoreJobID(42), notFound.JobID)

	require.ErrorAs(t, holder.CancelRestore(42), &notFound)
}

func TestJobNotFoundError_Error(t *testing.T) {
	t.Parallel()

	err := NewJobNotFoundError(model.RestoreJobID(42))

	assert.Equal(t, "restore job with ID 42 not found", err.Error())
}

func TestSumRestoreRuns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		runs []model.RestoreRun
		want model.RestoreRun
	}{
		{
			name: "counters and throughput add up",
			runs: []model.RestoreRun{
				{
					Counters: model.RestoreCounters{ReadRecords: 1, InsertedRecords: 1},
					Metrics:  &models.Metrics{RecordsPerSecond: 10, KilobytesPerSecond: 100},
				},
				{
					Counters: model.RestoreCounters{ReadRecords: 2},
					Metrics:  &models.Metrics{RecordsPerSecond: 5, KilobytesPerSecond: 50},
				},
			},
			want: model.RestoreRun{
				Counters: model.RestoreCounters{ReadRecords: 3, InsertedRecords: 1},
				Metrics:  &models.Metrics{RecordsPerSecond: 15, KilobytesPerSecond: 150},
			},
		},
		{
			name: "a run with no throughput adds none",
			runs: []model.RestoreRun{
				{Counters: model.RestoreCounters{ReadRecords: 1}},
				{Counters: model.RestoreCounters{ReadRecords: 2}, Metrics: &models.Metrics{RecordsPerSecond: 5}},
			},
			want: model.RestoreRun{
				Counters: model.RestoreCounters{ReadRecords: 3},
				Metrics:  &models.Metrics{RecordsPerSecond: 5},
			},
		},
		{name: "no runs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			handlers := make([]RestoreHandler, 0, len(tt.runs))
			for _, run := range tt.runs {
				handler := NewMockRestoreHandler(ctrl)
				handler.EXPECT().Status().Return(run)
				handlers = append(handlers, handler)
			}

			assert.Equal(t, tt.want, sumRestoreRuns(handlers))
		})
	}
}
