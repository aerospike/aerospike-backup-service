package service

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/aerospike/backup/pkg/model"
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

func (h *JobsHolder) getStatus(jobID int) (*model.RestoreJobStatus, error) {
	h.Lock()
	defer h.Unlock()
	jobStatus, exists := h.restoreJobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job with ID %d not found", jobID)
	}
	copyJob := *jobStatus
	return &copyJob, nil
}

func (h *JobsHolder) increaseStats(jobID int, new *model.RestoreResult) {
	h.Lock()
	defer h.Unlock()
	current, found := h.restoreJobs[jobID]
	if found {
		current.TotalBytes += new.TotalBytes
		current.TotalRecords += new.TotalRecords
		current.ExpiredRecords += new.ExpiredRecords
		current.SkippedRecords += new.SkippedRecords
		current.IgnoredRecords += new.IgnoredRecords
		current.InsertedRecords += new.InsertedRecords
		current.ExistedRecords += new.ExistedRecords
		current.FresherRecords += new.FresherRecords
		current.IndexCount += new.IndexCount
		current.UDFCount += new.UDFCount
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
		current.Error = err.Error()
	}
}
