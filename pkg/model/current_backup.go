package model

import (
	"time"

	"github.com/aerospike/backup-go"
)

type CurrentBackups struct {
	Full        *CurrentBackup `json:"full,omitempty"`
	Incremental *CurrentBackup `json:"incremental,omitempty"`
}

type CurrentBackup struct {
	TotalRecords     uint64     `json:"total_records,omitempty"`
	DoneRecords      uint64     `json:"done_records,omitempty"`
	StartTime        time.Time  `json:"start_time,omitempty"`
	PercentageDone   int        `json:"percentage_done,omitempty"`
	EstimatedEndTime *time.Time `json:"estimated_end_time,omitempty"`
}

func NewCurrentBackup(handlers map[string]*backup.BackupHandler) *CurrentBackup {
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

	startTime := GetAnyHandler(handlers).GetStats().StartTime

	return &CurrentBackup{
		TotalRecords:     total,
		DoneRecords:      done,
		StartTime:        startTime,
		PercentageDone:   int(percent * 100),
		EstimatedEndTime: calculateEstimatedEndTime(startTime, percent),
	}
}

func GetAnyHandler(m map[string]*backup.BackupHandler) *backup.BackupHandler {
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
