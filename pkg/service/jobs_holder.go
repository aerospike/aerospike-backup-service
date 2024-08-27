package service

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/internal/server/dto"
)

type jobInfo struct {
	handlers     []RestoreHandler
	status       dto.JobStatus
	err          error
	totalRecords uint64
	startTime    time.Time
}

type JobsHolder struct {
	sync.Mutex
	restoreJobs map[dto.RestoreJobID]*jobInfo
}

// NewJobsHolder returns a new JobsHolder.
func NewJobsHolder() *JobsHolder {
	return &JobsHolder{
		restoreJobs: make(map[dto.RestoreJobID]*jobInfo),
	}
}

// newJob creates a new restore job and return its id.
func (h *JobsHolder) newJob() dto.RestoreJobID {
	// #nosec G404
	id := dto.RestoreJobID(rand.Int())
	h.Lock()
	defer h.Unlock()
	h.restoreJobs[id] = &jobInfo{
		status:    dto.JobStatusRunning,
		startTime: time.Now(),
	}
	return id
}

// addHandler should be called for each backup (full or incremental) handler.
func (h *JobsHolder) addHandler(id dto.RestoreJobID, handler RestoreHandler) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		job.handlers = append(job.handlers, handler)
	}
}

// addTotalRecords should be called once for each namespace in the beginning
// of the restore process.
func (h *JobsHolder) addTotalRecords(id dto.RestoreJobID, t uint64) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		job.totalRecords += t
	}
}

func (h *JobsHolder) setDone(id dto.RestoreJobID) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		job.status = dto.JobStatusDone
	}
}

func (h *JobsHolder) setFailed(id dto.RestoreJobID, err error) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		job.status = dto.JobStatusFailed
		job.err = err
	}
}

func (h *JobsHolder) getStatus(id dto.RestoreJobID) (*dto.RestoreJobStatus, error) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.restoreJobs[id]; exists {
		return RestoreJobStatus(job), nil
	}
	return nil, fmt.Errorf("job with ID %d not found", id)
}
