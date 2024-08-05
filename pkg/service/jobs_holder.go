package service

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/aerospike/backup/pkg/model"
)

type RestoreJobID int

type JobsHolder struct {
	sync.Mutex
	restoreJobs map[RestoreJobID]*model.RestoreJobStatus
}

func NewJobsHolder() *JobsHolder {
	return &JobsHolder{
		restoreJobs: make(map[RestoreJobID]*model.RestoreJobStatus),
	}
}

func (h *JobsHolder) newJob() RestoreJobID {
	// #nosec G404
	jobID := RestoreJobID(rand.Int())
	h.Lock()
	defer h.Unlock()
	h.restoreJobs[jobID] = model.NewRestoreJobStatus()
	return jobID
}

func (h *JobsHolder) getStatus(jobID RestoreJobID) (*model.RestoreJobStatus, error) {
	h.Lock()
	defer h.Unlock()
	jobStatus, exists := h.restoreJobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job with ID %d not found", jobID)
	}
	copyJob := *jobStatus
	return &copyJob, nil
}

func (h *JobsHolder) increaseStats(jobID RestoreJobID, newStats *model.RestoreResult) {
	h.Lock()
	defer h.Unlock()
	current, found := h.restoreJobs[jobID]
	if found {
		current.TotalBytes += newStats.TotalBytes
		current.TotalRecords += newStats.TotalRecords
		current.ExpiredRecords += newStats.ExpiredRecords
		current.SkippedRecords += newStats.SkippedRecords
		current.IgnoredRecords += newStats.IgnoredRecords
		current.InsertedRecords += newStats.InsertedRecords
		current.ExistedRecords += newStats.ExistedRecords
		current.FresherRecords += newStats.FresherRecords
		current.IndexCount += newStats.IndexCount
		current.UDFCount += newStats.UDFCount
	}
}

func (h *JobsHolder) setDone(jobID RestoreJobID) {
	h.Lock()
	defer h.Unlock()
	current, found := h.restoreJobs[jobID]
	if found {
		current.Status = model.JobStatusDone
	}
}

func (h *JobsHolder) setFailed(jobID RestoreJobID, err error) {
	h.Lock()
	defer h.Unlock()
	current, found := h.restoreJobs[jobID]
	if found {
		current.Status = model.JobStatusFailed
		current.Error = err.Error()
	}
}
