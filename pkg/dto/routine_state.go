package dto

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// RoutineState represent the current state of backups (full and incremental)
// @Description RoutineState represent the current state of backups (full and incremental).
type RoutineState struct {
	// Full represents the state of a full backup. Nil if no full backup is running.
	Full *RunningJob `json:"full,omitempty"`
	// Incremental represents the state of an incremental backup. Nil if no incremental backup is running.
	Incremental *RunningJob `json:"incremental,omitempty"`
	// LastFull: the time of the last successful full backup.
	// A nil value indicates that there has never been a full backup.
	LastFull *time.Time `json:"last-full,omitempty"`
	// LastIncremental: the time of the last successful incremental backup.
	// A nil value indicates that there has never been an incremental backup.
	LastIncremental *time.Time `json:"last-incremental,omitempty"`
	// NextFull: the time of the next scheduled full backup.
	NextFull *time.Time `json:"next-full,omitempty"`
	// NextIncremental: the time of the next scheduled incremental backup.
	NextIncremental *time.Time `json:"next-incremental,omitempty"`
}

func NewRoutineStateFromModel(m *model.RoutineState) *RoutineState {
	if m == nil {
		return nil
	}

	c := &RoutineState{}
	c.fromModel(m)
	return c
}

func (c *RoutineState) fromModel(m *model.RoutineState) {
	c.Full = NewRunningJobFromModel(m.Full)
	c.Incremental = NewRunningJobFromModel(m.Incremental)
	c.LastFull = m.LastRunTime.FullBackupTime()
	c.LastIncremental = m.LastRunTime.IncrementalBackupTime()
	c.NextFull = m.NextRunTime.FullBackupTime()
	c.NextIncremental = m.NextRunTime.IncrementalBackupTime()
}

// RunningJob tracks progress of currently running job.
// @Description RunningJob tracks progress of currently running job.
type RunningJob struct {
	// The total number of records to be processed.
	TotalRecords uint64 `json:"total-records" example:"100"`
	// The number of records that have been successfully done.
	DoneRecords uint64 `json:"done-records" example:"50"`
	// The time when the operation started.
	StartTime time.Time `json:"start-time" example:"2006-01-02T15:04:05Z07:00"`
	// The time when the operation finished.
	// A nil value indicates that the operation is still running.
	FinishTime *time.Time `json:"finish-time,omitempty" example:"2006-01-02T15:04:05Z07:00"`
	// The progress of the backup operation as a percentage.
	// For backup jobs, this value can exceed 100% if:
	//   * new data is written to the database during the backup, or
	//   * the estimated total record count is lower than the actual count.
	PercentageDone uint `json:"percentage-done" example:"50"`
	// The estimated time when the backup operation will be completed.
	// It is calculated based on the current percentage done and duration.
	// A nil value indicates that the estimation is not available yet.
	// This value is not guaranteed to be accurate and may even be earlier than
	// the current time if PercentageDone exceeds 100%.
	EstimatedEndTime *time.Time `json:"estimated-end-time,omitempty" example:"2006-01-02T15:04:05Z07:00"`
	// Metrics provides real-time information about data flow performance.
	Metrics *Metrics `json:"metrics,omitempty"`
}

func NewRunningJobFromModel(m *model.RunningJob) *RunningJob {
	if m == nil {
		return nil
	}

	r := &RunningJob{}
	r.fromModel(m)
	return r
}

func (r *RunningJob) fromModel(m *model.RunningJob) {
	r.TotalRecords = m.TotalRecords
	r.DoneRecords = m.DoneRecords
	r.StartTime = m.StartTime
	r.FinishTime = m.FinishTime
	r.PercentageDone = m.PercentageDone
	r.EstimatedEndTime = m.EstimatedEndTime
	r.Metrics = NewMetricsFromModel(m.Metrics)
}
