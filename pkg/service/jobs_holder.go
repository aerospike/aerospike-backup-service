package service

import (
	"math/rand"
	"sync"
)

type JobsHolder struct {
	sync.Mutex
	restoreJobs map[int]string
}

func NewJobsHolder() *JobsHolder {
	return &JobsHolder{
		restoreJobs: make(map[int]string),
	}
}

func (h *JobsHolder) newJob() int {
	jobID := rand.Int()
	h.Lock()
	defer h.Unlock()
	h.restoreJobs[jobID] = jobStatusRunning
	return jobID
}

func (h *JobsHolder) getStatus(jobID int) string {
	h.Lock()
	defer h.Unlock()
	jobStatus, ok := h.restoreJobs[jobID]
	if !ok {
		return jobStatusNA
	}
	return jobStatus
}

func (h *JobsHolder) setDone(jobID int) {
	h.Lock()
	defer h.Unlock()
	h.restoreJobs[jobID] = jobStatusDone
}

func (h *JobsHolder) setFailed(jobID int) {
	h.Lock()
	defer h.Unlock()
	h.restoreJobs[jobID] = jobStatusFailed
}
