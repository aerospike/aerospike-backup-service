package service

import (
	"fmt"
	"github.com/aerospike/aerospike-backup-service/pkg/model"
	"math/rand"
	"sync"
	"time"
)

type jobInfo struct {
	handlers     []RestoreHandler
	status       model.JobStatus
	err          error
	totalRecords uint64
	startTime    time.Time
}

type JobsHolder struct {
	sync.Mutex
	restoreJobs map[model.RestoreJobID]*jobInfo
}

// NewJobsHolder returns a new JobsHolder.
func NewJobsHolder() *JobsHolder {
	return &JobsHolder{
		restoreJobs: make(map[model.RestoreJobID]*jobInfo),
	}
}

// newJob creates a new restore job and return its id.
func (h *JobsHolder) newJob() model.RestoreJobID {
	// #nosec G404
	id := model.RestoreJobID(rand.Int())
	h.Lock()
	defer h.Unlock()
	h.restoreJobs[id] = &jobInfo{
		status:    model.JobStatusRunning,
		startTime: time.Now(),
	}
	return id
}

// addHandler should be called for each backup (full or incremental) handler.
func (h *JobsHolder) addHandler(id model.RestoreJobID, handler RestoreHandler) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		job.handlers = append(job.handlers, handler)
	}
}

// addTotalRecords should be called once for each namespace in the beginning
// of the restore process.
func (h *JobsHolder) addTotalRecords(id model.RestoreJobID, t uint64) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		job.totalRecords += t
	}
}

func (h *JobsHolder) setDone(id model.RestoreJobID) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		job.status = model.JobStatusDone
	}
}

func (h *JobsHolder) setFailed(id model.RestoreJobID, err error) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		job.status = model.JobStatusFailed
		job.err = err
	}
}

func (h *JobsHolder) getStatus(id model.RestoreJobID) (*model.RestoreJobStatus, error) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		return RestoreJobStatus(job), nil
	}
	return nil, fmt.Errorf("job with ID %d not found", id)
}
