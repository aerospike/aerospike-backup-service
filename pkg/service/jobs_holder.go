package service

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
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
	sync.Mutex
	jobs map[model.RestoreJobID]*jobInfo
}

// NewRestoreJobsHolder returns a new RestoreJobsHolder.
func NewRestoreJobsHolder() *RestoreJobsHolder {
	return &RestoreJobsHolder{
		jobs: make(map[model.RestoreJobID]*jobInfo),
	}
}

// newJob creates a new restore job and return its id.
func (h *RestoreJobsHolder) newJob(label string) model.RestoreJobID {
	// #nosec G404
	id := model.RestoreJobID(rand.Int())
	h.Lock()
	defer h.Unlock()

	h.jobs[id] = &jobInfo{
		status:    model.JobStatusRunning,
		startTime: time.Now(),
		label:     label,
	}
	return id
}

// addJob should be called for each backup (full or incremental) handler.
func (h *RestoreJobsHolder) addJob(id model.RestoreJobID, handler *RestoreHandlerWithCancel) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.jobs[id]; exists {
		job.handlers = append(job.handlers, handler)
	}
}

// addTotalRecords should be called once for each namespace in the beginning
// of the restore process.
func (h *RestoreJobsHolder) addTotalRecords(id model.RestoreJobID, t uint64) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.jobs[id]; exists {
		job.totalRecords += t
	}
}

func (h *RestoreJobsHolder) finishJob(id model.RestoreJobID, err error) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.jobs[id]; exists {
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
	}
}

func (h *RestoreJobsHolder) getStatus(id model.RestoreJobID) (*model.RestoreJobStatus, error) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.jobs[id]; exists {
		return RestoreJobStatus(job), nil
	}
	return nil, NewErrJobNotFound(id)
}

func (h *RestoreJobsHolder) getJob(id model.RestoreJobID) (*jobInfo, error) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.jobs[id]; exists {
		return job, nil
	}
	return nil, NewErrJobNotFound(id)
}
