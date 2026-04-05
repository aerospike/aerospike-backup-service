package model

import (
	"strings"

	"github.com/aerospike/backup-go/models"
)

type RestoreState string

const (
	RestoreRunning  RestoreState = "running"
	RestoreSuccess  RestoreState = "success"
	RestoreFailure  RestoreState = "failure"
	RestoreCanceled RestoreState = "canceled"
)

// AllRestoreStatuses returns all defined restore job statuses.
func AllRestoreStatuses() []RestoreState {
	return []RestoreState{
		RestoreRunning,
		RestoreSuccess,
		RestoreFailure,
		RestoreCanceled,
	}
}

// ParseRestoreState parses a restore status from user input (e.g. query parameters).
// It accepts canonical values (running, success, failure, canceled) case-insensitively,
// and legacy aliases done and failed for success and failure.
func ParseRestoreState(s string) (RestoreState, bool) {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "done":
		return RestoreSuccess, true
	case "failed":
		return RestoreFailure, true
	}
	for _, st := range AllRestoreStatuses() {
		if strings.EqualFold(s, string(st)) {
			return st, true
		}
	}

	return "", false
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
