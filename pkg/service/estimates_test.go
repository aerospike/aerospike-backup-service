package service

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCurrentBackupStatus(t *testing.T) {
	t.Parallel()

	// A minute in: the end time is start + elapsed/progress.
	start := time.Now().Add(-time.Minute)
	metrics := &models.Metrics{RecordsPerSecond: 7}
	tests := map[string]struct {
		run                 model.BackupRun
		wantProgress        float64
		wantDone, wantTotal uint64
		wantMetrics         *models.Metrics
		wantDuration        time.Duration
	}{
		"scan backup passes its counts through": {
			run: model.BackupRun{
				Progress: 0.4,
				Scan:     &model.ScanProgress{TotalRecords: 100, DoneRecords: 40, Metrics: metrics},
			},
			wantProgress: 0.4, wantDone: 40, wantTotal: 100, wantMetrics: metrics,
			wantDuration: 150 * time.Second,
		},
		"a run with no scan counts no records": {
			run:          model.BackupRun{Progress: 0.4},
			wantProgress: 0.4, wantDuration: 150 * time.Second,
		},
		"progress and end time are capped while running": {
			run: model.BackupRun{
				Progress: 1,
				Scan:     &model.ScanProgress{TotalRecords: 10, DoneRecords: 10, Metrics: metrics},
			},
			wantProgress: 0.99, wantDone: 10, wantTotal: 10, wantMetrics: metrics,
			wantDuration: time.Minute * 100 / 99,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			run := tt.run
			run.StartTime = start
			handler := NewMockRoutineBackupHandler(gomock.NewController(t))
			handler.EXPECT().Status().Return(&run)

			job := currentBackupStatus(handler)

			require.NotNil(t, job)
			assert.Equal(t, tt.wantDone, job.DoneRecords)
			assert.Equal(t, tt.wantTotal, job.TotalRecords)
			assert.InDelta(t, tt.wantProgress, job.Progress, 1e-9)
			assert.Equal(t, start, job.StartTime)
			assert.Same(t, tt.wantMetrics, job.Metrics)
			require.NotNil(t, job.EstimatedEndTime)
			// Tolerance well under the 0.6s an uncapped estimate would add.
			assert.WithinDuration(t, start.Add(tt.wantDuration), *job.EstimatedEndTime, 100*time.Millisecond)
		})
	}
}

func TestCurrentBackupStatus_NotStarted(t *testing.T) {
	t.Parallel()

	handler := NewMockRoutineBackupHandler(gomock.NewController(t))
	handler.EXPECT().Status().Return(nil)

	assert.Nil(t, currentBackupStatus(handler))
}

// TestCurrentBackupStatus_ScanOperation runs a real scan operation through currentBackupStatus,
// so the REST job is checked against backup-go's stats rather than a hand-built BackupRun.
func TestCurrentBackupStatus_ScanOperation(t *testing.T) {
	t.Parallel()

	stats := models.NewBackupStats()
	stats.TotalRecords.Store(100)
	stats.ReadRecords.Store(40)
	stats.StartTime = time.Now().Add(-time.Minute)
	h := NewMockCancelableBackupHandler(gomock.NewController(t))
	h.EXPECT().GetStats().Return(stats)
	h.EXPECT().GetMetrics().Return(&models.Metrics{RecordsPerSecond: 7})
	op := &BackupNamespacesOperation{handlers: map[string]CancelableBackupHandler{"ns1": h}}

	job := currentBackupStatus(op)

	require.NotNil(t, job)
	assert.Equal(t, uint64(40), job.DoneRecords)
	assert.Equal(t, uint64(100), job.TotalRecords)
	assert.InDelta(t, 0.4, job.Progress, 1e-9)
	assert.Equal(t, stats.StartTime, job.StartTime)
	require.NotNil(t, job.Metrics)
	assert.Equal(t, uint64(7), job.Metrics.RecordsPerSecond)
	assert.NotNil(t, job.EstimatedEndTime)
}

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
