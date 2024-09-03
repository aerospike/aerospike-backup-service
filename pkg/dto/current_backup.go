package dto

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

// CurrentBackups represent the current state of backups (full and incremental)
type CurrentBackups struct {
	// Full represents the state of a full backup. Nil if no full backup is running.
	Full *RunningJob `json:"full,omitempty"`
	// Incremental represents the state of an incremental backup. Nil if no incremental backup is running.
	Incremental *RunningJob `json:"incremental,omitempty"`
}

func NewCurrentBackupsFromModel(m *model.CurrentBackups) *CurrentBackups {
	if m == nil {
		return nil
	}

	c := &CurrentBackups{}
	c.fromModel(m)
	return c
}

func (c *CurrentBackups) fromModel(m *model.CurrentBackups) {
	c.Full = NewRunningJobFromModel(m.Full)
	c.Incremental = NewRunningJobFromModel(m.Incremental)
}

// RunningJob tracks progress of currently running job.
// @Description RunningJob tracks progress of currently running job.
type RunningJob struct {
	// TotalRecords: the total number of records to be processed.
	TotalRecords uint64 `json:"total-records,omitempty" example:"100"`
	// DoneRecords: the number of records that have been successfully done.
	DoneRecords uint64 `json:"done-records,omitempty" example:"50"`
	// StartTime: the time when the backup operation started.
	StartTime time.Time `json:"start-time,omitempty" example:"2006-01-02T15:04:05Z07:00"`
	// PercentageDone: the progress of the backup operation as a percentage.
	PercentageDone uint `json:"percentage-done,omitempty" example:"50"`
	// EstimatedEndTime: the estimated time when the backup operation will be completed.
	// A nil value indicates that the estimation is not available yet.
	EstimatedEndTime *time.Time `json:"estimated-end-time,omitempty" example:"2006-01-02T15:04:05Z07:00"`
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
	r.PercentageDone = m.PercentageDone
	r.EstimatedEndTime = m.EstimatedEndTime
}
