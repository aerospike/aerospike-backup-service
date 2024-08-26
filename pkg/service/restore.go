package service

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/pkg/dto"
)

type RestoreManager interface {
	// Restore starts a restore process using the given request.
	// Returns the job id as a unique identifier.
	Restore(request *dto.RestoreRequestInternal) (dto.RestoreJobID, error)

	// RestoreByTime starts a restore by time process using the given request.
	// Returns the job id as a unique identifier.
	RestoreByTime(request *dto.RestoreTimestampRequest) (dto.RestoreJobID, error)

	// JobStatus returns status for the given job id.
	JobStatus(jobID dto.RestoreJobID) (*dto.RestoreJobStatus, error)

	// RetrieveConfiguration return backed up Aerospike configuration.
	RetrieveConfiguration(routine string, toTime time.Time) ([]byte, error)
}
