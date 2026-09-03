package model

import (
	"github.com/aerospike/backup-go/models"
)

// RestoreState represents possible states of restore jobs.
type RestoreState string

const (
	RestoreRunning  RestoreState = "running"
	RestoreSuccess  RestoreState = "success"
	RestoreFailure  RestoreState = "failure"
	RestoreCanceled RestoreState = "canceled"
)

// String returns the wire value of the restore job status.
func (s RestoreState) String() string {
	return string(s)
}

// RestoreJobStatus represents a restore job status.
// The information included depends on the Status field:
//   - RestoreRunning -> current statistics and estimation.
//   - RestoreSuccess -> statistics.
//   - RestoreFailure -> error.
type RestoreJobStatus struct {
	Counters       *models.RestoreStats
	CurrentRestore *RunningJob
	Status         RestoreState
	Error          error
}
