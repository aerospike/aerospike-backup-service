package model

import (
	"time"
)

// CurrentBackups represent the current state of backups (full and incremental)
type CurrentBackups struct {
	// Full represents the state of a full backup. Nil if no full backup is running.
	Full *RunningJob
	// Incremental represents the state of an incremental backup. Nil if no incremental backup is running.
	Incremental *RunningJob
}

// RunningJob tracks progress of currently running job.
// @Description RunningJob tracks progress of currently running job.
type RunningJob struct {
	// TotalRecords: the total number of records to be processed.
	TotalRecords uint64
	// DoneRecords: the number of records that have been successfully done.
	DoneRecords uint64
	// StartTime: the time when the backup operation started.
	StartTime time.Time
	// PercentageDone: the progress of the backup operation as a percentage.
	PercentageDone uint
	// EstimatedEndTime: the estimated time when the backup operation will be completed.
	// A nil value indicates that the estimation is not available yet.
	EstimatedEndTime *time.Time
}
