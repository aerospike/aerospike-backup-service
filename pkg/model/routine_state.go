package model

import (
	"time"

	"github.com/aerospike/backup-go/models"
)

// RoutineState represent the current state of backups (full and incremental).
type RoutineState struct {
	// Full represents the state of a full backup. Nil if no full backup is running.
	Full *RunningJob
	// Incremental represents the state of an incremental backup. Nil if no incremental backup is running.
	Incremental *RunningJob
	// LastRunTime contains information about the latest run time for both full and incremental backups.
	LastRunTime *BackupTime
	// NextRunTime specifies the scheduled next execution time for the routine's backup operations.
	NextRunTime *BackupTime
}

// RunningJob tracks progress of currently running job.
type RunningJob struct {
	// TotalRecords: the total number of records to be processed.
	// For backup jobs, this value is only an estimate, not exact.
	TotalRecords uint64
	// DoneRecords: the number of records that have been successfully done.
	DoneRecords uint64
	// StartTime: the time when the operation started.
	StartTime time.Time
	// FinishTime: the time when the operation finished.
	// nil value indicates that operation is still running.
	FinishTime *time.Time
	// PercentageDone is the current progress of the operation as a percentage.
	// For backup jobs, this value can exceed 100% if:
	//   * new data is written to the database during the backup, or
	//   * the estimated total record count is lower than the actual count.
	PercentageDone uint
	// EstimatedEndTime: the estimated time when the operation will be completed.
	// A nil value indicates that the estimation is not available yet.
	// This value is not guaranteed to be accurate and may even be earlier than
	// the current time if PercentageDone exceeds 100%.
	EstimatedEndTime *time.Time
	// Metrics contains performance metrics for the current operation.
	Metrics *models.Metrics
}
