package service

import "github.com/aerospike/backup/pkg/model"

type RestoreService interface {
	// Restore starts a restore process using the given request.
	// Returns the job id as a unique identifier.
	Restore(request *model.RestoreRequest) int

	// RestoreByTime starts a restore by time process using the given request.
	// Returns the job id as a unique identifier.
	RestoreByTime(request *model.RestoreRequest) int

	// JobStatus returns status for the given job id.
	JobStatus(jobID int) string
}
