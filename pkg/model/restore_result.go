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
// The information included depends on the Status field:
//   - JobStatusRunning -> current statistics and estimation.
//   - JobStatusDone -> statistics.
//   - JobStatusFailed -> error.
type RestoreJobStatus struct {
	Counters       *models.RestoreStats
	CurrentRestore *RunningJob
	Status         JobStatus
	Error          error
}
