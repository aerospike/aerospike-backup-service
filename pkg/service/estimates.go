package service

import (
	"time"

	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup/pkg/model"
)

func currentBackupStatus(handlers map[string]*backup.BackupHandler) *model.RunningJob {
	if len(handlers) == 0 {
		return nil
	}

	var total, done uint64
	for _, handler := range handlers {
		done += handler.GetStats().GetReadRecords()
		total += handler.GetStats().TotalRecords
	}
	if total == 0 {
		return nil
	}
	percent := float64(done) / float64(total)

	startTime := getAnyHandler(handlers).GetStats().StartTime

	return &model.RunningJob{
		TotalRecords:     total,
		DoneRecords:      done,
		StartTime:        startTime,
		PercentageDone:   uint(percent * 100),
		EstimatedEndTime: calculateEstimatedEndTime(startTime, percent),
	}
}

func getAnyHandler(m map[string]*backup.BackupHandler) *backup.BackupHandler {
	for _, value := range m {
		return value
	}

	return nil
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

func CurrentRestoreStatus(job *jobInfo) *model.RestoreJobStatus {
	currentStatus := &model.RestoreJobStatus{
		Status: job.status,
	}

	for _, handler := range job.handlers {
		stats := handler.GetStats()
		currentStatus.ReadRecords += stats.GetReadRecords()
		currentStatus.InsertedRecords += stats.GetRecordsInserted()
		currentStatus.IndexCount += uint64(stats.GetSIndexes())
		currentStatus.UDFCount += uint64(stats.GetUDFs())
		currentStatus.FresherRecords += stats.GetRecordsFresher()
		currentStatus.SkippedRecords += stats.GetRecordsSkipped()
		currentStatus.ExistedRecords += stats.GetRecordsExisted()
		currentStatus.ExpiredRecords += stats.GetRecordsExpired()
		currentStatus.TotalBytes += stats.GetTotalBytesRead()
	}

	percentage := float64(currentStatus.ReadRecords) / float64(job.totalRecords)
	currentStatus.EstimatedEndTime = calculateEstimatedEndTime(job.startTime, percentage)
	currentStatus.PercentageDone = uint(percentage * 100)
	currentStatus.StartTime = job.startTime

	return currentStatus
}
