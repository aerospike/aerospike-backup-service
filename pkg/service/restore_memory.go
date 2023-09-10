package service

import (
	"math/rand"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
)

const (
	JOB_STATUS_NA      = "NA"
	JOB_STATUS_RUNNING = "RUNNING"
	JOB_STATUS_DONE    = "DONE"
	JOB_STATUS_FAILED  = "FAILED"
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
	jobId := rand.Int() // TODO: use a request hash code
	go func() {
		r.restoreService.RestoreRun(request)
		r.restoreJobs[jobId] = JOB_STATUS_DONE
	}()
	r.restoreJobs[jobId] = JOB_STATUS_RUNNING
	return jobId
}

func (r *RestoreMemory) JobStatus(jobId int) string {
	jobStatus, ok := r.restoreJobs[jobId]
	if !ok {
		return JOB_STATUS_NA
	}
	return jobStatus
}
