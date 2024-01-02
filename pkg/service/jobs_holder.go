package service

import (
	"github.com/aerospike/backup/pkg/model"
	"math/rand"
	"sync"
)

type JobsHolder struct {
	sync.Mutex
	restoreJobs map[int]*model.RestoreJobStatus
}

func NewJobsHolder() *JobsHolder {
	return &JobsHolder{
		restoreJobs: make(map[int]*model.RestoreJobStatus),
	}
}

func (h *JobsHolder) newJob() int {
	jobID := rand.Int()
	h.Lock()
	defer h.Unlock()
	h.restoreJobs[jobID] = model.NewRestoreJobStatus()
	return jobID
}

func (h *JobsHolder) getStatus(jobID int) *model.RestoreJobStatus {
	h.Lock()
	defer h.Unlock()
	return h.restoreJobs[jobID]
}

func (h *JobsHolder) increaseStats(jobID int, new *model.RestoreResult) {
	h.Lock()
	defer h.Unlock()
	current, found := h.restoreJobs[jobID]
	if found {
		current.Bytes += new.Bytes
		current.Number += new.Number
	}
}
func (h *JobsHolder) setDone(jobID int) {
	h.Lock()
	defer h.Unlock()
	current, found := h.restoreJobs[jobID]
	if found {
		current.Status = model.JobStatusDone
	}
}

func (h *JobsHolder) setFailed(jobID int, err error) {
	h.Lock()
	defer h.Unlock()
	current, found := h.restoreJobs[jobID]
	if found {
		current.Status = model.JobStatusFailed
		current.Error = err
	}
}
