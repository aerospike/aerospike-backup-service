package service

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
)

// restoreJob encapsulates the state and details of a single restore operation.
// A restore job can consist multiple backups (one full backup and several incremental) across multiple namespaces.
type restoreJob struct {
	// handlers contains the list of active restore handlers from the underlying backup library.
	// Each handler corresponds to a specific backup being restored.
	handlers []restoreexecutor.RestoreHandler

	// status indicates the current state of the job (Running, Done, Failed, Cancelled).
	status model.JobStatus

	// err holds all errors that occurred during the job execution.
	// It is nil if the job is running or completed successfully.
	err error

	// totalRecords is the aggregated count of records to be restored across all backups.
	totalRecords uint64

	// started is the timestamp marking the initiation of the restore job.
	started time.Time

	// finished is the timestamp marking the completion of the restore job.
	// It is nil for jobs that are still running.
	finished *time.Time

	// label provides a user-friendly name for the restore job, used for metrics.
	label string

	// cancel is a function that cancels the context of the restore job,
	// effectively stopping the operation.
	cancel context.CancelFunc
}
type RestoreJobsHolder struct {
	*util.SafeMap[model.RestoreJobID, *restoreJob]
}

// NewRestoreJobsHolder returns a new RestoreJobsHolder.
func NewRestoreJobsHolder() *RestoreJobsHolder {
	return &RestoreJobsHolder{
		SafeMap: util.NewSafeMap[model.RestoreJobID, *restoreJob](),
	}
}

// newJob creates a new restore job and return its id.
func (h *RestoreJobsHolder) newJob(label string, cancel context.CancelFunc) model.RestoreJobID {
	// #nosec G404
	id := model.RestoreJobID(rand.Int63())
	h.Store(id, &restoreJob{
		status:  model.JobStatusRunning,
		started: time.Now(),
		label:   label,
		cancel:  cancel,
	})

	return id
}

// addHandler should be called for each backup (full or incremental) handler.
func (h *RestoreJobsHolder) addHandler(id model.RestoreJobID, handler restoreexecutor.RestoreHandler) {
	h.Apply(id, func(job *restoreJob) {
		job.handlers = append(job.handlers, handler)
	})
}

// addTotalRecords should be called once for each namespace in the beginning
// of the restore process.
func (h *RestoreJobsHolder) addTotalRecords(id model.RestoreJobID, t uint64) {
	h.Apply(id, func(job *restoreJob) {
		job.totalRecords += t
	})
}

func (h *RestoreJobsHolder) finishJob(id model.RestoreJobID, err error) {
	h.Apply(id, func(job *restoreJob) {
		job.finished = util.Ptr(time.Now())
		if err == nil {
			job.status = model.JobStatusDone
			return
		}
		if errors.Is(err, context.Canceled) {
			job.status = model.JobStatusCancelled
			job.err = err
			return
		}
		job.status = model.JobStatusFailed
		job.err = err
		slog.Error("Failed to restore", slog.Any("jobId", id), slog.Any("err", err))
	})
}

func (h *RestoreJobsHolder) getStatus(id model.RestoreJobID) (*model.RestoreJobStatus, error) {
	var result *model.RestoreJobStatus
	h.Apply(id, func(value *restoreJob) {
		result = RestoreJobStatus(value)
	})

	if result != nil {
		return result, nil
	}
	return nil, NewErrJobNotFound(id)
}

func (h *RestoreJobsHolder) getJob(id model.RestoreJobID) (*restoreJob, error) {
	if job, exists := h.Load(id); exists {
		return job, nil
	}
	return nil, NewErrJobNotFound(id)
}
