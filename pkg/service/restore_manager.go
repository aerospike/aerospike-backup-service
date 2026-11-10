package service

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
)

// RestoreManager starts restore jobs and keeps their status in memory, so a job can be
// queried or canceled by its id. Statuses live as long as the process. Each backup mode has its
// own RestoreManager, built on a [RestoreJobsHolder].
type RestoreManager interface {
	// Restore starts a restore process using the given request and returns the job id.
	// The job runs in the background for the lifetime of the service.
	Restore(request *model.RestoreRequest) model.RestoreJobID

	// RestoreByTime starts a restore by time process using the given request and returns
	// the job id. The job runs in the background for the lifetime of the service.
	RestoreByTime(request *model.RestoreTimestampRequest) model.RestoreJobID

	// JobStatus returns status for the given job id.
	JobStatus(jobID model.RestoreJobID) (*model.RestoreJobStatus, error)

	// CancelRestore cancels an ongoing restore.
	CancelRestore(jobID model.RestoreJobID) error

	// GetFilteredJobs returns all jobs matching the given filters as a map of jobId -> RestoreJobStatus.
	GetFilteredJobs(
		timeBounds model.TimeBounds,
		statusFilter model.StatusFilter,
	) map[model.RestoreJobID]*model.RestoreJobStatus
}

type JobNotFoundError struct {
	JobID model.RestoreJobID
}

func (e *JobNotFoundError) Error() string {
	return fmt.Sprintf("restore job with ID %d not found", e.JobID)
}

func NewJobNotFoundError(id model.RestoreJobID) *JobNotFoundError {
	return &JobNotFoundError{JobID: id}
}

// RestoreLogAttrs returns the counters of a finished restore and how long it took, as log attributes.
func RestoreLogAttrs(c model.RestoreCounters, duration time.Duration) []slog.Attr {
	return []slog.Attr{
		slog.Uint64("recordsInserted", c.InsertedRecords),
		slog.Uint64("recordsExpired", c.ExpiredRecords),
		slog.Uint64("recordsSkipped", c.SkippedRecords),
		slog.Uint64("recordsIgnored", c.IgnoredRecords),
		slog.Uint64("recordsFresher", c.FresherRecords),
		slog.Uint64("recordsExisted", c.ExistedRecords),
		slog.Uint64("totalBytesRead", c.TotalBytesRead),
		slog.Uint64("errorsInDoubt", c.ErrorsInDoubt),
		slog.Uint64("readRecords", c.ReadRecords),
		slog.Uint64("secondaryIndexes", c.SecondaryIndexes),
		slog.Uint64("udFs", c.UDFs),
		slog.Uint64("bytesWritten", c.BytesWritten),
		slog.Duration("duration", duration),
	}
}
