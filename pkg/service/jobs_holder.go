package service

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
)

type jobInfo struct {
	handlers     []*RestoreHandlerWithCancel // Each handler restores one namespace.
	status       model.JobStatus
	err          error
	totalRecords uint64
	startTime    time.Time
	label        string
}

type RestoreJobsHolder struct {
	*util.SafeMap[model.RestoreJobID, *jobInfo]
}

// NewRestoreJobsHolder returns a new RestoreJobsHolder.
func NewRestoreJobsHolder() *RestoreJobsHolder {
	return &RestoreJobsHolder{
		SafeMap: util.NewSafeMap[model.RestoreJobID, *jobInfo](),
	}
}

// newJob creates a new restore job and return its id.
func (h *RestoreJobsHolder) newJob(label string) model.RestoreJobID {
	// #nosec G404
	id := model.RestoreJobID(rand.Int63())
	h.Store(id, &jobInfo{
		status:    model.JobStatusRunning,
		startTime: time.Now(),
		label:     label,
	},
	)

	return id
}

// addHandler should be called for each backup (full or incremental) handler.
func (h *RestoreJobsHolder) addHandler(id model.RestoreJobID, handler *RestoreHandlerWithCancel) {
	h.Apply(id, func(job *jobInfo) {
		job.handlers = append(job.handlers, handler)
	})
}

// addTotalRecords should be called once for each namespace in the beginning
// of the restore process.
func (h *RestoreJobsHolder) addTotalRecords(id model.RestoreJobID, t uint64) {
	h.Apply(id, func(job *jobInfo) {
		job.totalRecords += t
	})
}

func (h *RestoreJobsHolder) finishJob(id model.RestoreJobID, err error) {
	h.Apply(id, func(job *jobInfo) {
		if err == nil {
			job.status = model.JobStatusDone
			return
		}
		if errors.Is(err, context.Canceled) {
			job.status = model.JobStatusCancelled
			return
		}
		job.status = model.JobStatusFailed
		job.err = err
	})
}

func (h *RestoreJobsHolder) getStatus(id model.RestoreJobID) (*model.RestoreJobStatus, error) {
	var result *model.RestoreJobStatus
	h.Apply(id, func(value *jobInfo) {
		result = RestoreJobStatus(value)
	})

	if result != nil {
		return result, nil
	}
	return nil, NewErrJobNotFound(id)
}

func (h *RestoreJobsHolder) getJob(id model.RestoreJobID) (*jobInfo, error) {
	if job, exists := h.Load(id); exists {
		return job, nil
	}
	return nil, NewErrJobNotFound(id)
}
