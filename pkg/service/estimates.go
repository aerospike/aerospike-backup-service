package service

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/backup-go/models"
)

// currentBackupStatus is the state of the backup the handler runs, or nil when none is running or
// it has not started. Progress is capped at 99% while the backup runs. The record counts and
// metrics are those of a scan backup; a run without a scan slot has none.
func currentBackupStatus(handler RoutineBackupHandler) *model.RunningJob {
	if handler == nil {
		return nil
	}

	// A handler is registered while its backup runs, so having one means the backup is running.
	run := handler.Status()
	if run == nil { // registered, not started yet
		return nil
	}

	progress := min(run.Progress, 0.99)
	job := &model.RunningJob{
		StartTime:        run.StartTime,
		EstimatedEndTime: calculateEstimatedEndTime(run.StartTime, progress),
		Progress:         progress,
	}
	if run.Scan != nil {
		job.DoneRecords = run.Scan.DoneRecords
		job.TotalRecords = run.Scan.TotalRecords
		job.Metrics = run.Scan.Metrics
	}

	return job
}

// NewRestoreRunningJob creates a RunningJob for a restore, with its percentage and estimated end
// time. reported is the progress the restore run reports itself, as a fraction. When it is nil,
// the progress is done out of total.
func NewRestoreRunningJob(
	startTime time.Time,
	finishTime *time.Time,
	done, total uint64,
	reported *float64,
	metrics *models.Metrics,
	jobStatus model.RestoreState,
) *model.RunningJob {
	fraction, known := restoreProgress(done, total, reported)
	if !known { // an empty restore, with nothing to measure the progress against.
		return &model.RunningJob{
			StartTime:  startTime,
			FinishTime: finishTime,
		}
	}

	var (
		progress         float64
		endTime          *time.Time
		effectiveMetrics *models.Metrics
	)

	switch jobStatus {
	case model.RestoreRunning:
		progress = min(fraction, 0.99) // percentage should not exceed 99%.
		endTime = calculateEstimatedEndTime(startTime, progress)
		effectiveMetrics = metrics
	case model.RestoreSuccess:
		progress = 1.0 // 100% only for successfully finished jobs.
	default:
	}

	return &model.RunningJob{
		StartTime:        startTime,
		FinishTime:       finishTime,
		DoneRecords:      done,
		TotalRecords:     total,
		EstimatedEndTime: endTime,
		Progress:         progress,
		Metrics:          effectiveMetrics,
	}
}

// restoreProgress returns the fraction of a restore done, and false when it cannot be told: the
// run reports none and there is no total to measure done against.
func restoreProgress(done, total uint64, reported *float64) (float64, bool) {
	if reported != nil {
		return *reported, true
	}
	if total == 0 {
		return 0, false
	}

	return float64(done) / float64(total), true
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
