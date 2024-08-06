package service

import (
	"time"

	"github.com/aerospike/backup/pkg/model"
)

type RestoreManager interface {
	// Restore starts a restore process using the given request.
	// Returns the job id as a unique identifier.
	Restore(request *model.RestoreRequestInternal) (RestoreJobID, error)

	// RestoreByTime starts a restore by time process using the given request.
	// Returns the job id as a unique identifier.
	RestoreByTime(request *model.RestoreTimestampRequest) (RestoreJobID, error)

	// JobStatus returns status for the given job id.
	JobStatus(jobID RestoreJobID) (*model.RestoreJobStatus, error)

	// RetrieveConfiguration return backed up Aerospike configuration.
	RetrieveConfiguration(routine string, toTime time.Time) ([]byte, error)
}
