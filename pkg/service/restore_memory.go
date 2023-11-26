package service

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/aerospike/backup/pkg/util"
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
	restoreJobs    *JobsHolder
	restoreService shared.Restore
	backends       map[string]BackupBackend
}

// NewRestoreMemory returns a new RestoreMemory instance.
func NewRestoreMemory(backends map[string]BackupBackend) *RestoreMemory {
	return &RestoreMemory{
		restoreJobs:    NewJobsHolder(),
		restoreService: shared.NewRestore(),
		backends:       backends,
	}
}

func (r *RestoreMemory) doRestore(request *model.RestoreRequest) bool {
	var success bool
	restoreRunFunc := func() {
		success = r.restoreService.RestoreRun(request)
	}
	out := stdIO.Capture(restoreRunFunc)
	util.LogCaptured(out)
	return success
}

// Restore starts the backup for a given request asynchronously and
// returns the id of the backup job.
func (r *RestoreMemory) Restore(request *model.RestoreRequest) int {
	jobID := r.restoreJobs.newJob()
	go func() {
		restoreResult := r.doRestore(request)
		if restoreResult {
			r.restoreJobs.setDone(jobID)
		} else {
			r.restoreJobs.setFailed(jobID)
		}
	}()
	return jobID
}

func (r *RestoreMemory) RestoreByTime(request *model.RestoreRequest) int {
	jobID := r.restoreJobs.newJob()
	go func() {
		backend := r.backends[request.Routine]
		fullBackup, err := r.findLastFullBackup(backend, request, jobID)
		if err != nil {
			slog.Error(err.Error())
			r.restoreJobs.setFailed(jobID)
			return
		}

		if !r.restoreFullBackup(request, fullBackup.Key) {
			slog.Error("Could not restore full backup", "routine", request.Routine)
			r.restoreJobs.setFailed(jobID)
			return
		}

		incrementalBackups, err := r.findIncrementalBackups(backend, *fullBackup.LastModified)
		if err != nil {
			slog.Error(err.Error())
			r.restoreJobs.setFailed(jobID)
			return

		}

		if failed := r.restoreIncrementalBackups(incrementalBackups, request); failed != nil {
			slog.Error("Could not restore incremental backup", "routine", request.Routine, "key", failed)
			r.restoreJobs.setFailed(jobID)
			return
		}

		r.restoreJobs.setDone(jobID)
	}()
	return jobID
}

func (r *RestoreMemory) restoreIncrementalBackups(incrementalBackups []model.BackupDetails, request *model.RestoreRequest) *string {
	for _, b := range incrementalBackups {
		incrRestoreOK := r.doRestore(&model.RestoreRequest{
			DestinationCuster: request.DestinationCuster,
			SourceStorage:     request.SourceStorage,
			Policy:            request.Policy,
			Directory:         nil,
			File:              b.Key,
		})

		if incrRestoreOK == false {
			return b.Key
		}
	}
	return nil
}

func (r *RestoreMemory) findIncrementalBackups(backend BackupBackend, u time.Time) ([]model.BackupDetails, error) {
	allIncrementalBackupList, err := backend.IncrementalBackupList()
	if err != nil {
		return nil, err
	}
	var filteredIncrementalBackups []model.BackupDetails
	for _, b := range allIncrementalBackupList {
		if b.LastModified.After(u) {
			filteredIncrementalBackups = append(filteredIncrementalBackups, b)
		}
	}
	return filteredIncrementalBackups, nil
}

func (r *RestoreMemory) restoreFullBackup(request *model.RestoreRequest, key *string) bool {
	return r.doRestore(&model.RestoreRequest{
		DestinationCuster: request.DestinationCuster,
		SourceStorage:     request.SourceStorage,
		Policy:            request.Policy,
		Directory:         key,
		File:              nil,
	})
}

func (r *RestoreMemory) findLastFullBackup(backend BackupBackend,
	request *model.RestoreRequest, jobID int) (*model.BackupDetails, error) {
	fullBackupList, err := backend.FullBackupList()
	if err != nil {
		slog.Error("cannot read full backup list", "name", request.Routine)
		r.restoreJobs.setFailed(jobID)
		return nil, fmt.Errorf("cannot read full backup list for %s", request.Routine)
	}

	fullBackup := latestFullBackup(fullBackupList, time.UnixMilli(request.Time))
	if fullBackup == nil {
		r.restoreJobs.setFailed(jobID)
		return nil, fmt.Errorf("no full backup found for %s at %d", request.Routine, request.Time)
	}

	return fullBackup, nil
}

func latestFullBackup(list []model.BackupDetails, time time.Time) *model.BackupDetails {
	var latestFullBackup *model.BackupDetails
	for _, b := range list {
		if b.LastModified.Before(time) {
			if latestFullBackup == nil || latestFullBackup.LastModified.After(*b.LastModified) {
				latestFullBackup = &b
			}
		}
	}
	return latestFullBackup
}

// JobStatus returns the status of the job with the given id.
func (r *RestoreMemory) JobStatus(jobID int) string {
	return r.restoreJobs.getStatus(jobID)
}
