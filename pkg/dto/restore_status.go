package dto

import (
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

var jobStatuses = []JobStatus{RestoreRunning, RestoreSuccess, RestoreFailure, RestoreCanceled}

const (
	deprecatedJobStatusDone   = "done"
	deprecatedJobStatusFailed = "failed"
)

// Deprecated query-param aliases; not part of the public JobStatus enum.
var deprecatedJobStatuses = []JobStatus{deprecatedJobStatusDone, deprecatedJobStatusFailed}

// Validate checks that the job status is supported.
func (s JobStatus) Validate() error {
	if c, ok := canonicalEnum(s, jobStatuses); ok && c != "" {
		return nil
	}
	if c, ok := canonicalEnum(s, deprecatedJobStatuses); ok && c != "" {
		return nil
	}

	return errValidationInvalidValue("status", s, jobStatuses)
}

// ToModel converts the DTO job status to the model type.
func (s JobStatus) ToModel() model.RestoreState {
	alias, _ := canonicalEnum(s, deprecatedJobStatuses)
	if alias == deprecatedJobStatusFailed {
		return model.RestoreFailure
	}
	if alias == deprecatedJobStatusDone {
		return model.RestoreSuccess
	}

	c, _ := canonicalEnum(s, jobStatuses)
	return model.RestoreState(c)
}

// NewJobStatusFromModel creates a DTO job status from the model type.
func NewJobStatusFromModel(m model.RestoreState) JobStatus {
	return JobStatus(m)
}
