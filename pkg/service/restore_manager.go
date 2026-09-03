package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/backup-go/models"
)

// RestoreManager starts restore jobs and keeps their status in memory, so a job can be
// queried or canceled by its id. Statuses live as long as the process.
type RestoreManager interface {
	// Restore starts a restore process using the given request.
	// Returns the job id as a unique identifier.
	Restore(ctx context.Context, request *model.RestoreRequest) (model.RestoreJobID, error)

	// RestoreByTime starts a restore by time process using the given request.
	// Returns the job id as a unique identifier.
	RestoreByTime(ctx context.Context, request *model.RestoreTimestampRequest) (model.RestoreJobID, error)

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
	restoreService restoreexecutor.Restore,
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

func (r *restoreManager) Restore(ctx context.Context, request *model.RestoreRequest) (model.RestoreJobID, error) {
	return r.pathRunner.Restore(ctx, request)
}

func (r *restoreManager) RestoreByTime(
	ctx context.Context, request *model.RestoreTimestampRequest,
) (model.RestoreJobID, error) {
	return r.timeRunner.RestoreByTime(ctx, request)
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

func logAttrs(s *models.RestoreStats) []slog.Attr {
	return []slog.Attr{
		slog.Int64("recordsInserted", int64(s.GetRecordsInserted())),
		slog.Int64("recordsExpired", int64(s.GetRecordsExpired())),
		slog.Int64("recordsSkipped", int64(s.GetRecordsSkipped())),
		slog.Int64("recordsIgnored", int64(s.GetRecordsIgnored())),
		slog.Int64("recordsFresher", int64(s.GetRecordsFresher())),
		slog.Int64("recordsExisted", int64(s.GetRecordsExisted())),
		slog.Int64("totalBytesRead", int64(s.TotalBytesRead.Load())),
		slog.Int64("errorsInDoubt", int64(s.GetErrorsInDoubt())),
		slog.Int64("readRecords", int64(s.ReadRecords.Load())),
		slog.Int("secondaryIndexes", int(s.GetSIndexes())),
		slog.Int("udFs", int(s.GetUDFs())),
		slog.Int64("bytesWritten", int64(s.BytesWritten.Load())),
		slog.Duration("duration", s.GetDuration()),
	}
}
