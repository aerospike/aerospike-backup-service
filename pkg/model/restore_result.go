package model

import "github.com/aerospike/backup-go/models"

type RestoreState string

const (
	RestoreRunning  RestoreState = "running"
	RestoreDone     RestoreState = "done"
	RestoreFailed   RestoreState = "failed"
	RestoreCanceled RestoreState = "canceled"
)

// AllJobStatuses returns all defined restore job statuses.
func AllJobStatuses() []RestoreState {
	return []RestoreState{
		RestoreRunning,
		RestoreDone,
		RestoreFailed,
		RestoreCanceled,
	}
}

// RestoreJobStatus represents a restore job status.
// The information included depends on the Status field:
//   - RestoreRunning -> current statistics and estimation.
//   - RestoreDone -> statistics.
//   - RestoreFailed -> error.
type RestoreJobStatus struct {
	Counters       *models.RestoreStats
	CurrentRestore *RunningJob
	Status         RestoreState
	Error          error
}
