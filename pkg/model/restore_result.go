package model

import "github.com/aerospike/backup-go/models"

type JobStatus string

const (
	JobStatusRunning   JobStatus = "Running"
	JobStatusDone      JobStatus = "Done"
	JobStatusFailed    JobStatus = "Failed"
	JobStatusCancelled JobStatus = "Cancelled"
)

// RestoreJobStatus represents a restore job status.
type RestoreJobStatus struct {
	Counters       *models.RestoreStats
	CurrentRestore *RunningJob
	Status         JobStatus
	Error          error
}
