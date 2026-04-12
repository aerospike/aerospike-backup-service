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

	var done = stats.ReadRecords.Load()
	var total = stats.TotalRecords.Load()

	percentage := 0.0
	if total != 0 {
		percentage = min(float64(done)/float64(total), 0.99) // percentage should not exceed 99%.
	}

	endTime := calculateEstimatedEndTime(stats.StartTime, percentage)

	return &model.RunningJob{
		StartTime:        stats.StartTime,
		FinishTime:       nil,
		DoneRecords:      done,
		TotalRecords:     total,
		EstimatedEndTime: endTime,
		PercentageDone:   uint(percentage * 100),
		Metrics:          handlers.GetMetrics(),
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
		percentage       float64
		endTime          *time.Time
		effectiveMetrics *models.Metrics
	)

	switch jobStatus {
	case model.RestoreRunning:
		percentage = min(float64(done)/float64(total), 0.99) // percentage should not exceed 99%.
		endTime = calculateEstimatedEndTime(startTime, percentage)
		effectiveMetrics = metrics
	case model.RestoreSuccess:
		percentage = 1.0 // 100% only for successfully finished jobs.
	default:
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
