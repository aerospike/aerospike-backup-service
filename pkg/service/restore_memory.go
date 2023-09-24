package service

import (
	"math/rand"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
)

const (
	jobStatusNA      = "NA"
	jobStatusRunning = "RUNNING"
	jobStatusDone    = "DONE"
	jobStatusFailed  = "FAILED"
)

// RestoreMemory implements the RestoreService interface.
// Stores job information locally within a map.
type RestoreMemory struct {
	restoreJobs    map[int]string
	restoreService shared.Restore
}

// NewRestoreMemory returns a new RestoreMemory instance.
func NewRestoreMemory() *RestoreMemory {
	return &RestoreMemory{
		restoreJobs:    make(map[int]string),
		restoreService: shared.NewRestore(),
	}
}

func (r *RestoreMemory) Restore(request *model.RestoreRequest) int {
	jobID := rand.Int() // TODO: use a request hash code
	go func() {
		r.restoreService.RestoreRun(request)
		r.restoreJobs[jobID] = jobStatusDone
	}()
	r.restoreJobs[jobID] = jobStatusRunning
	return jobID
}

func (r *RestoreMemory) JobStatus(jobID int) string {
	jobStatus, ok := r.restoreJobs[jobID]
	if !ok {
		return jobStatusNA
	}
	return jobStatus
}
