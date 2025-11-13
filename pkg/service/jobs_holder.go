package service

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

// restoreJob encapsulates the state and details of a single restore operation.
// A restore job can consist multiple backups (one full backup and several incremental) across multiple namespaces.
type restoreJob struct {
	sync.RWMutex

	// The following fields are protected by the mutex and should only be accessed under lock.

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

	// finished is the timestamp marking the completion of the restore job.
	// It is nil for jobs that are still running.
	finished *time.Time

	// Immutable fields, set at creation:

	// started is the timestamp marking the initiation of the restore job.
	started time.Time

	// label provides a user-friendly name for the restore job, used for metrics.
	label string

	// cancel is a function that cancels the context of the restore job,
	// effectively stopping the operation.
	cancel context.CancelFunc
}

// addHandler adds a new restore handler to the job.
func (j *restoreJob) addHandler(handler restoreexecutor.RestoreHandler) {
	j.Lock()
	defer j.Unlock()

	j.handlers = append(j.handlers, handler)
}

// addTotalRecords adds to the total records count for the job.
func (j *restoreJob) addTotalRecords(t uint64) {
	j.Lock()
	defer j.Unlock()

	j.totalRecords += t
}

// finish marks the job as finished with a given status and error.
func (j *restoreJob) finish(err error) {
	j.Lock()
	defer j.Unlock()

	j.finished = ptr.Of(time.Now())
	j.err = err

	switch {
	case err == nil:
		j.status = model.JobStatusDone
	case errors.Is(err, context.Canceled):
		j.status = model.JobStatusCancelled
	default:
		j.status = model.JobStatusFailed
	}
}

// buildStatus constructs a model.RestoreJobStatus from the current job state.
func (j *restoreJob) buildStatus() *model.RestoreJobStatus {
	j.RLock()
	defer j.RUnlock()

	return RestoreJobStatus(j)
}

// RestoreJobsHolder is a thread-safe map of restore jobs.
type RestoreJobsHolder struct {
	*collections.SafeMap[model.RestoreJobID, *restoreJob]
}

// NewRestoreJobsHolder returns a new RestoreJobsHolder.
func NewRestoreJobsHolder() *RestoreJobsHolder {
	return &RestoreJobsHolder{
		SafeMap: collections.NewSafeMap[model.RestoreJobID, *restoreJob](),
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
	if job, ok := h.Load(id); ok {
		job.addHandler(handler)
	}
}

// addTotalRecords should be called once for each namespace in the beginning
// of the restore process.
func (h *RestoreJobsHolder) addTotalRecords(id model.RestoreJobID, t uint64) {
	if job, ok := h.Load(id); ok {
		job.addTotalRecords(t)
	}
}

func (h *RestoreJobsHolder) finishJob(id model.RestoreJobID, err error) {
	if job, ok := h.Load(id); ok {
		job.finish(err)
	}
}

func (h *RestoreJobsHolder) getStatus(id model.RestoreJobID) (*model.RestoreJobStatus, error) {
	job, exists := h.Load(id)
	if !exists {
		return nil, NewErrJobNotFound(id)
	}
	// buildStatus handles its own locking
	return job.buildStatus(), nil
}

func (h *RestoreJobsHolder) getJob(id model.RestoreJobID) (*restoreJob, error) {
	if job, exists := h.Load(id); exists {
		return job, nil
	}
	return nil, NewErrJobNotFound(id)
}
