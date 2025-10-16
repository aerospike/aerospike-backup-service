package service

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/models"
)

func currentBackupStatus(handlers CancelableBackupHandler) *model.RunningJob {
	if handlers == nil {
		return nil
	}

	stats := handlers.GetStats()
	if stats == nil { // no running jobs
		return nil
	}

	return NewRunningJob(
		stats.StartTime, nil, stats.ReadRecords.Load(), stats.TotalRecords.Load(), handlers.GetMetrics(), true)
}

// RestoreJobStatus returns the status of a restore job.
// The information included in the response depends on the job status:
//   - model.JobStatusRunning -> current statistics and estimation.
//   - model.JobStatusDone -> statistics.
//   - status model.JobStatusFailed -> error.
func RestoreJobStatus(job *restoreJob) *model.RestoreJobStatus {
	metrics := make([]*models.Metrics, 0, len(job.handlers))
	stats := make([]*models.RestoreStats, 0, len(job.handlers))
	for _, handler := range job.handlers {
		metrics = append(metrics, handler.GetMetrics())
		stats = append(stats, handler.GetStats())
	}

	restoreStats := models.SumRestoreStats(stats...)

	doneRecords := restoreStats.GetReadRecords()
	sumMetrics := models.SumMetrics(metrics...)
	runningJob := NewRunningJob(
		job.started, job.finished, doneRecords, job.totalRecords, sumMetrics, job.status == model.JobStatusRunning)

	return &model.RestoreJobStatus{
		Status:         job.status,
		Error:          job.err,
		Counters:       restoreStats,
		CurrentRestore: runningJob,
	}
}

// NewRunningJob created new RunningJob (backup or restore) with calculated estimated time and percentage.
func NewRunningJob(
	startTime time.Time,
	finishTime *time.Time,
	done, total uint64,
	metrics *models.Metrics,
	withEstimates bool,
) *model.RunningJob {
	if total == 0 { // edge case for empty restore
		return &model.RunningJob{
			StartTime:  startTime,
			FinishTime: finishTime,
		}
	}

	percentage := min(float64(done)/float64(total), 1.0) // percentage should not exceed 100%.
	var (
		endTime          *time.Time
		effectiveMetrics *models.Metrics
	)
	if withEstimates {
		endTime = calculateEstimatedEndTime(startTime, percentage)
		effectiveMetrics = metrics
	}

	return &model.RunningJob{
		StartTime:        startTime,
		FinishTime:       finishTime,
		DoneRecords:      done,
		TotalRecords:     total,
		EstimatedEndTime: endTime,
		PercentageDone:   uint(percentage * 100),
		Metrics:          effectiveMetrics,
	}
}

func calculateEstimatedEndTime(startTime time.Time, percentDone float64) *time.Time {
	if percentDone < 0.01 { // too early to calculate estimation, or zero done yet.
		return nil
	}

	elapsed := time.Since(startTime)
	totalTime := time.Duration(float64(elapsed) / percentDone)
	result := startTime.Add(totalTime)
	return &result
}
