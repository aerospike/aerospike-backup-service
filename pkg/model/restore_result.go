package model

import (
	"github.com/aerospike/backup-go/models"
)

type RestoreState int

const (
	RestoreRunning RestoreState = iota
	RestoreSuccess
	RestoreFailure
	RestoreCanceled
)

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
