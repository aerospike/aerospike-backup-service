package service

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

type RestoreManager interface {
	// Restore starts a restore process using the given request.
	// Returns the job id as a unique identifier.
	Restore(request *model.RestoreRequest) (model.RestoreJobID, error)

	// RestoreByTime starts a restore by time process using the given request.
	// Returns the job id as a unique identifier.
	RestoreByTime(request *model.RestoreTimestampRequest) (model.RestoreJobID, error)

	// JobStatus returns status for the given job id.
	JobStatus(jobID model.RestoreJobID) (*model.RestoreJobStatus, error)

	// RetrieveConfiguration return backed up Aerospike configuration.
	RetrieveConfiguration(routine string, toTime time.Time) ([]byte, error)
}
