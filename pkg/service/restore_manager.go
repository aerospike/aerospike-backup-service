package service

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
)

// RestoreManager starts restore jobs and keeps their status in memory, so a job can be
// queried or canceled by its id. Statuses live as long as the process.
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

// restoreManager stores restore job information locally and delegates execution by request type.
type restoreManager struct {
	restoreJobs *RestoreJobsHolder
	pathRunner  *pathRestoreRunner
	timeRunner  *timeRestoreRunner
}

var _ RestoreManager = (*restoreManager)(nil)

// NewRestoreManager returns a RestoreManager.
func NewRestoreManager(
	restoreService Restore,
	clientManager aerospike.ClientManager,
	restoreJobs *RestoreJobsHolder,
	backupReader BackupReader,
	routineStorage *collections.LockMap,
	validator RestoreValidator,
) RestoreManager {
	return &restoreManager{
		restoreJobs: restoreJobs,
		pathRunner: newPathRestoreRunner(
			restoreJobs,
			restoreService,
			backupReader,
			clientManager,
			routineStorage,
			validator,
		),
		timeRunner: newTimeRestoreRunner(
			restoreJobs,
			restoreService,
			backupReader,
			clientManager,
			routineStorage,
			validator,
		),
	}
}

func (r *restoreManager) Restore(request *model.RestoreRequest) model.RestoreJobID {
	return r.pathRunner.Restore(request)
}

func (r *restoreManager) RestoreByTime(request *model.RestoreTimestampRequest) model.RestoreJobID {
	return r.timeRunner.RestoreByTime(request)
}

// JobStatus returns the status of the job with the given id.
func (r *restoreManager) JobStatus(jobID model.RestoreJobID) (*model.RestoreJobStatus, error) {
	job, err := r.restoreJobs.getJob(jobID)
	if err != nil {
		return nil, err
	}

	return job.buildStatus(), nil
}

// CancelRestore cancels an ongoing restore.
func (r *restoreManager) CancelRestore(jobID model.RestoreJobID) error {
	job, err := r.restoreJobs.getJob(jobID)
	if err != nil {
		return err
	}

	job.cancel()

	return nil
}

// GetFilteredJobs returns all jobs matching the given filters as a map of jobId -> RestoreJobStatus.
func (r *restoreManager) GetFilteredJobs(
	timeBounds model.TimeBounds,
	statusFilter model.StatusFilter,
) map[model.RestoreJobID]*model.RestoreJobStatus {
	results := make(map[model.RestoreJobID]*model.RestoreJobStatus)

	r.restoreJobs.Iterate(func(id model.RestoreJobID, job *restoreJob) {
		if !timeBounds.Contains(job.started) {
			return
		}

		// Build the status first to get a consistent snapshot of the job.
		// This prevents a race condition where the job's status changes
		// between filtering and building the result.
		status := job.buildStatus()

		// Now, filter based on the consistent snapshot.
		if !statusFilter.Matches(status.Status) {
			return
		}

		results[id] = status
	})

	return results
}

// logAttrs returns the counters of a finished restore and how long it took, as log attributes.
func logAttrs(c model.RestoreCounters, duration time.Duration) []slog.Attr {
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
