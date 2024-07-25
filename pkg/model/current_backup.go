package model

import "time"

type CurrentBackup struct {
	TotalRecords     uint64    `json:"total_records,omitempty"`
	DoneRecords      uint64    `json:"done_records,omitempty"`
	StartTime        time.Time `json:"start_time,omitempty"`
	PercentageDone   int       `json:"percentage_done,omitempty"`
	EstimatedEndTime time.Time `json:"estimated_end_time,omitempty"`
}

func NewCurrentBackup(startTime time.Time, done, total uint64) *CurrentBackup {
	if total == 0 {
		return nil
	}

	percent := float64(done) / float64(total)

	return &CurrentBackup{
		TotalRecords:     total,
		DoneRecords:      done,
		StartTime:        startTime,
		PercentageDone:   int(percent * 100),
		EstimatedEndTime: calculateEstimatedEndTime(startTime, percent),
	}
}

func calculateEstimatedEndTime(startTime time.Time, percentDone float64) time.Time {
	elapsed := time.Since(startTime)
	totalTime := time.Duration(float64(elapsed) * 100 / percentDone)
	return startTime.Add(totalTime)
}
