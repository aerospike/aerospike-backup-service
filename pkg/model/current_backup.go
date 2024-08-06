package model

import (
	"time"
)

// CurrentBackups represent the current state of backups (full and incremental)
type CurrentBackups struct {
	// Full represents the state of a full backup. Nil if no full backup is running.
	Full *CurrentBackup `json:"full,omitempty"`
	// Incremental represents the state of an incremental backup. Nil if no incremental backup is running.
	Incremental *CurrentBackup `json:"incremental,omitempty"`
}

type CurrentBackup struct {
	// TotalRecords: the total number of records to be backed up.
	TotalRecords uint64 `json:"total-records,omitempty" example:"100"`
	// DoneRecords: the number of records that have been successfully backed up.
	DoneRecords uint64 `json:"done-records,omitempty" example:"50"`
	// StartTime: the time when the backup operation started.
	StartTime time.Time `json:"start-time,omitempty" example:"2006-01-02T15:04:05Z07:00"`
	// PercentageDone: the progress of the backup operation as a percentage.
	PercentageDone int `json:"percentage-done,omitempty" example:"50"`
	// EstimatedEndTime: the estimated time when the backup operation will be completed.
	// A nil value indicates that the estimation is not available yet.
	EstimatedEndTime *time.Time `json:"estimated-end-time,omitempty" example:"2006-01-02T15:04:05Z07:00"`
}
