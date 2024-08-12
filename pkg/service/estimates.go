package service

import (
	"time"

	"github.com/aerospike/backup/pkg/model"
)

func currentBackupStatus(handlers map[string]BackupHandler) *model.RunningJob {
	if len(handlers) == 0 {
		return nil
	}

	var total, done uint64
	for _, handler := range handlers {
		done += handler.GetStats().GetReadRecords()
		total += handler.GetStats().TotalRecords
	}

	// These are the backups of multiple namespaces in the same routine.
	// Therefore, picking any of those is valid, since they started at
	// the same time.
	startTime := getAnyHandler(handlers).GetStats().StartTime

	return NewRunningJob(startTime, done, total)
}

func getAnyHandler(m map[string]BackupHandler) BackupHandler {
	for _, value := range m {
		return value
	}

	return nil
}

// RestoreJobStatus returns the status of a restore job.
// The information included in the response depends on the job status:
//   - model.JobStatusRunning -> current statistics and estimation.
//   - model.JobStatusDone -> statistics.
//   - status model.JobStatusFailed -> error.
func RestoreJobStatus(job *jobInfo) *model.RestoreJobStatus {
	status := &model.RestoreJobStatus{
		Status: job.status,
	}

	for _, handler := range job.handlers {
		stats := handler.GetStats()
		status.ReadRecords += stats.GetReadRecords()
		status.InsertedRecords += stats.GetRecordsInserted()
		status.IndexCount += uint64(stats.GetSIndexes())
		status.UDFCount += uint64(stats.GetUDFs())
		status.FresherRecords += stats.GetRecordsFresher()
		status.SkippedRecords += stats.GetRecordsSkipped()
		status.ExistedRecords += stats.GetRecordsExisted()
		status.ExpiredRecords += stats.GetRecordsExpired()
		status.TotalBytes += stats.GetTotalBytesRead()
	}

	if job.status == model.JobStatusRunning {
		status.CurrentRestore = NewRunningJob(job.startTime, status.ReadRecords,
			job.totalRecords)
	}

	if job.err != nil {
		status.Error = job.err.Error()
	}

	return status
}

// NewRunningJob created new RunningJob with calculated estimated time and percentage.
func NewRunningJob(startTime time.Time, done, total uint64) *model.RunningJob {
	if total == 0 {
		return nil
	}

	percentage := float64(done) / float64(total)
	return &model.RunningJob{
		StartTime:        startTime,
		DoneRecords:      done,
		TotalRecords:     total,
		EstimatedEndTime: calculateEstimatedEndTime(startTime, percentage),
		PercentageDone:   uint(percentage * 100),
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
