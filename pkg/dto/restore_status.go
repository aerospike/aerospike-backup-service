package dto

import (
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// JobStatus represents possible states of restore jobs.
// @Description Possible states of restore jobs.
type JobStatus string

const ( // These consts are required as strings for swagger generation.
	RestoreRunning  JobStatus = "running"
	RestoreSuccess  JobStatus = "success"
	RestoreFailure  JobStatus = "failure"
	RestoreCanceled JobStatus = "canceled"
)

// AllJobStatuses returns all defined job statuses as strings.
func AllJobStatuses() []string {
	return []string{
		string(RestoreRunning),
		string(RestoreSuccess),
		string(RestoreFailure),
		string(RestoreCanceled),
	}
}

// ParseJobStatus parses a restore status from user input (e.g. query parameters).
// It accepts canonical values (running, success, failure, canceled) case-insensitively,
// and legacy aliases done and failed for success and failure.
func ParseJobStatus(s string) (JobStatus, bool) {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "done", string(RestoreRunning):
		return RestoreSuccess, true
	case "failed", string(RestoreFailure):
		return RestoreFailure, true
	case string(RestoreSuccess):
		return RestoreSuccess, true
	case string(RestoreCanceled):
		return RestoreCanceled, true
	}

	return "", false
}

// ToModel converts a DTO job status to the domain model state.
func (s JobStatus) ToModel() model.RestoreState {
	switch s {
	case RestoreRunning:
		return model.RestoreRunning
	case RestoreSuccess:
		return model.RestoreSuccess
	case RestoreFailure:
		return model.RestoreFailure
	case RestoreCanceled:
		return model.RestoreCanceled
	default:
		return -1
	}
}

// JobStatusFromModel converts a domain model state to DTO job status.
func JobStatusFromModel(rs model.RestoreState) JobStatus {
	switch rs {
	case model.RestoreRunning:
		return RestoreRunning
	case model.RestoreSuccess:
		return RestoreSuccess
	case model.RestoreFailure:
		return RestoreFailure
	case model.RestoreCanceled:
		return RestoreCanceled
	default:
		return ""
	}
}
