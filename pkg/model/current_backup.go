package model

import "time"

type CurrentBackup struct {
	StartTime        time.Time `json:"start_time,omitempty"`
	PercentageDone   int       `json:"percentage_done,omitempty"`
	EstimatedEndTime time.Time `json:"estimated_end_time,omitempty"`
}
