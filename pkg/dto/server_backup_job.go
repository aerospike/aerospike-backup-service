package dto

import (
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// ServerBackupJob is a server-side backup job of one namespace, as the server last reported it.
// @Description ServerBackupJob is a server-side backup job of one namespace, as the server last reported it.
type ServerBackupJob struct {
	// The server's job id.
	JobID string `json:"job-id" example:"261103T101500-0a1b"`
	// The namespace the job backed up.
	Namespace string `json:"namespace" example:"test"`
	// The server's job state, e.g. COMPLETE, FAILED or ABORTED.
	State string `json:"state" example:"COMPLETE"`
	// Why the job failed or was aborted, as the server reports it.
	ErrorReason string `json:"error-reason,omitempty" example:"S3 PUT failed: 403 Forbidden"`
	// What aborted the job: an operator request, a failure on a node or a cluster event.
	AbortOrigin string `json:"abort-origin,omitempty" enums:"operator,server,cluster" example:"operator"`
	// The time the job started.
	StartTime *time.Time `json:"start-time,omitempty" example:"2006-01-02T15:04:05Z07:00"`
	// The time the job finished.
	FinishTime *time.Time `json:"finish-time,omitempty" example:"2006-01-02T15:04:05Z07:00"`
	// The number of records backed up.
	RecordsBackedUp uint64 `json:"records-backed-up" example:"1000"`
	// The number of bytes backed up.
	BytesBackedUp uint64 `json:"bytes-backed-up" example:"1048576"`
	// The number of records the server could not read: a complete job that has any is missing them.
	RecordsReadFailed uint64 `json:"records-read-failed" example:"0"`
}

func newServerBackupJobsFromModel(jobs []model.ServerBackupJob) []ServerBackupJob {
	result := make([]ServerBackupJob, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, ServerBackupJob{
			JobID:             job.JobID,
			Namespace:         job.Namespace,
			State:             job.State,
			ErrorReason:       job.ErrorReason,
			AbortOrigin:       job.AbortOrigin,
			StartTime:         job.StartTime,
			FinishTime:        job.FinishTime,
			RecordsBackedUp:   job.RecordsBackedUp,
			BytesBackedUp:     job.BytesBackedUp,
			RecordsReadFailed: job.RecordsReadFailed,
		})
	}

	return result
}
