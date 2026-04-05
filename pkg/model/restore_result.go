package model

import (
	"strings"

	"github.com/aerospike/backup-go/models"
)

type RestoreState string

const (
	RestoreRunning  RestoreState = "running"
	RestoreDone     RestoreState = "done"
	RestoreFailed   RestoreState = "failed"
	RestoreCanceled RestoreState = "canceled"
)

// AllRestoreStatuses returns all defined restore job statuses.
func AllRestoreStatuses() []RestoreState {
	return []RestoreState{
		RestoreRunning,
		RestoreDone,
		RestoreFailed,
		RestoreCanceled,
	}
}

// ParseRestoreState parses a restore status from user input (e.g. query parameters).
// It accepts canonical values (running, done, failed, canceled) case-insensitively,
// and legacy aliases success and failure for done and failed.
func ParseRestoreState(s string) (RestoreState, bool) {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "success":
		return RestoreDone, true
	case "failure":
		return RestoreFailed, true
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
//   - RestoreDone -> statistics.
//   - RestoreFailed -> error.
type RestoreJobStatus struct {
	Counters       *models.RestoreStats
	CurrentRestore *RunningJob
	Status         RestoreState
	Error          error
}
