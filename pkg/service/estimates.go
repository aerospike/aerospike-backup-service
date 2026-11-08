package service

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/backupexecutor"
	"github.com/aerospike/backup-go/models"
)

func currentBackupStatus(handlers backupexecutor.BackupHandler) *model.RunningJob {
	if handlers == nil {
		return nil
	}

	// for backups, we don't store info on finished jobs; If we have a backup handler => it's running.
	stats := handlers.GetStats()
	if stats == nil { // no running jobs
		return nil
	}

	if reporter, ok := handlers.(ProgressReporter); ok {
		return runningJob(stats.StartTime, 0, 0, reporter.Progress(), handlers.GetMetrics())
	}

	done := stats.ReadRecords.Load()
	total := stats.TotalRecords.Load()

	progress := 0.0
	if total != 0 {
		progress = float64(done) / float64(total)
	}

	return runningJob(stats.StartTime, done, total, progress, handlers.GetMetrics())
}

// runningJob is the state of a backup still running, whose progress is capped at 99%. A server
// backup counts no records: done and total are zero and progress comes from the server.
func runningJob(startTime time.Time, done, total uint64, progress float64, metrics *models.Metrics) *model.RunningJob {
	progress = min(progress, 0.99)

	return &model.RunningJob{
		StartTime:        startTime,
		DoneRecords:      done,
		TotalRecords:     total,
		EstimatedEndTime: calculateEstimatedEndTime(startTime, progress),
		Progress:         progress,
		Metrics:          metrics,
	}
}

// NewRestoreRunningJob created new RunningJob for restore with calculated estimated time and percentage.
func NewRestoreRunningJob(
	startTime time.Time,
	finishTime *time.Time,
	done, total uint64,
	metrics *models.Metrics,
	jobStatus model.RestoreState,
) *model.RunningJob {
	if total == 0 { // edge case for empty restore
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
		progress = min(float64(done)/float64(total), 0.99) // percentage should not exceed 99%.
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

func calculateEstimatedEndTime(startTime time.Time, percentDone float64) *time.Time {
	if percentDone < 0.01 { // too early to calculate estimation, or zero done yet.
		return nil
	}

	elapsed := time.Since(startTime)
	totalTime := time.Duration(float64(elapsed) / percentDone)
	result := startTime.Add(totalTime)
	return &result
}
