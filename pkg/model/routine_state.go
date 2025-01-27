package model

import (
	"time"
)

// RoutineState represent the current state of backups (full and incremental).
type RoutineState struct {
	// Full represents the state of a full backup. Nil if no full backup is running.
	Full *RunningJob
	// Incremental represents the state of an incremental backup. Nil if no incremental backup is running.
	Incremental *RunningJob
	// LastRunTime contains information about the latest run time for both full and incremental backups.
	LastRunTime *LastBackupRun
}

// RunningJob tracks progress of currently running job.
type RunningJob struct {
	// TotalRecords: the total number of records to be processed.
	TotalRecords uint64
	// DoneRecords: the number of records that have been successfully done.
	DoneRecords uint64
	// StartTime: the time when the operation started.
	StartTime time.Time
	// FinishTime: the time when the operation finished.
	// nil value indicates that operation is still running.
	FinishTime *time.Time
	// PercentageDone: the progress of the operation as a percentage.
	PercentageDone uint
	// EstimatedEndTime: the estimated time when the operation will be completed.
	// A nil value indicates that the estimation is not available yet.
	EstimatedEndTime *time.Time
}
