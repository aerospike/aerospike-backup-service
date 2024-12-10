package service

import (
	"math/rand"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
)

type jobInfo struct {
	jobs         []*RestoreJob
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
func (h *RestoreJobsHolder) addJob(id model.RestoreJobID, handler *RestoreJob) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.jobs[id]; exists {
		job.jobs = append(job.jobs, handler)
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

func (h *RestoreJobsHolder) setDone(id model.RestoreJobID) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.jobs[id]; exists {
		job.status = model.JobStatusDone
	}
}

func (h *RestoreJobsHolder) setFailed(id model.RestoreJobID, err error) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.jobs[id]; exists {
		job.status = model.JobStatusFailed
		job.err = err
	}
}

func (h *RestoreJobsHolder) setCancelled(id model.RestoreJobID) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.jobs[id]; exists {
		job.status = model.JobStatusCancelled
	}
}

func (h *RestoreJobsHolder) getStatus(id model.RestoreJobID) (*model.RestoreJobStatus, error) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.jobs[id]; exists {
		return RestoreJobStatus(job), nil
	}
	return nil, &ErrJobNotFound{JobID: id}
}

func (h *RestoreJobsHolder) getJob(id model.RestoreJobID) (*jobInfo, error) {
	h.Lock()
	defer h.Unlock()
	if job, exists := h.jobs[id]; exists {
		return job, nil
	}
	return nil, &ErrJobNotFound{JobID: id}
}
