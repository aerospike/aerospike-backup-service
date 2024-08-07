package service

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/aerospike/backup/pkg/model"
)

type RestoreJobID int

type jobInfo struct {
	handlers     []RestoreHandler
	status       model.JobStatus
	err          error
	totalRecords uint64
	startTime    time.Time
}

type JobsHolder struct {
	sync.Mutex
	restoreJobs map[RestoreJobID]*jobInfo
}

func NewJobsHolder() *JobsHolder {
	return &JobsHolder{
		restoreJobs: make(map[RestoreJobID]*jobInfo),
	}
}

// newJob creates new restore job and return its id.
func (h *JobsHolder) newJob() RestoreJobID {
	// #nosec G404
	id := RestoreJobID(rand.Int())
	h.Lock()
	defer h.Unlock()
	h.restoreJobs[id] = &jobInfo{
		status:    model.JobStatusRunning,
		startTime: time.Now(),
	}
	return id
}

// addHandler should be called for each backup (full or incremental) handler.
func (h *JobsHolder) addHandler(id RestoreJobID, handler RestoreHandler) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		job.handlers = append(job.handlers, handler)
	}
}

// addTotalRecords should be called once for each namespace in the beginning of restore process.
func (h *JobsHolder) addTotalRecords(id RestoreJobID, t uint64) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		job.totalRecords += t
	}
}

func (h *JobsHolder) setDone(id RestoreJobID) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		job.status = model.JobStatusDone
	}
}

func (h *JobsHolder) setFailed(id RestoreJobID, err error) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		job.status = model.JobStatusFailed
		job.err = err
	}
}

func (h *JobsHolder) getStatus(id RestoreJobID) (*model.RestoreJobStatus, error) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		return RestoreJobStatus(job), nil
	}
	return nil, fmt.Errorf("job with ID %d not found", id)
}
