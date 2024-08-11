package dto

import (
	"time"

	"github.com/aerospike/backup/pkg/model"
)

// CurrentBackupsDTO represent the current state of backups (full and incremental)
type CurrentBackupsDTO struct {
	// Full represents the state of a full backup. Nil if no full backup is running.
	Full *RunningJobDTO `json:"full,omitempty"`
	// Incremental represents the state of an incremental backup. Nil if no incremental backup is running.
	Incremental *RunningJobDTO `json:"incremental,omitempty"`
}

// RunningJobDTO tracks progress of currently running job.
// @Description RunningJobDTO tracks progress of currently running job.
type RunningJobDTO struct {
	// TotalRecords: the total number of records to be processed.
	TotalRecords *uint64 `json:"total-records,omitempty" example:"100"`
	// DoneRecords: the number of records that have been successfully done.
	DoneRecords *uint64 `json:"done-records,omitempty" example:"50"`
	// StartTime: the time when the backup operation started.
	StartTime *time.Time `json:"start-time,omitempty" example:"2006-01-02T15:04:05Z07:00"`
	// PercentageDone: the progress of the backup operation as a percentage.
	PercentageDone *uint `json:"percentage-done,omitempty" example:"50"`
	// EstimatedEndTime: the estimated time when the backup operation will be completed.
	// A nil value indicates that the estimation is not available yet.
	EstimatedEndTime *time.Time `json:"estimated-end-time,omitempty" example:"2006-01-02T15:04:05Z07:00"`
}

func MapCurrentBackupsToDTO(b *model.CurrentBackups) CurrentBackupsDTO {
	if b == nil {
		return CurrentBackupsDTO{}
	}
	var dto CurrentBackupsDTO
	if b.Full != nil {
		dto.Full = mapRunningJobToDTO(*b.Full)
	}
	if b.Incremental != nil {
		dto.Incremental = mapRunningJobToDTO(*b.Incremental)
	}
	return dto
}

func mapRunningJobToDTO(j model.RunningJob) *RunningJobDTO {
	return &RunningJobDTO{
		TotalRecords:     &j.TotalRecords,
		DoneRecords:      &j.DoneRecords,
		StartTime:        &j.StartTime,
		PercentageDone:   &j.PercentageDone,
		EstimatedEndTime: j.EstimatedEndTime,
	}
}
