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
	config         *model.Config
	restoreJobs    *JobsHolder
	restoreService shared.Restore
	backends       map[string]BackupBackend
}

var _ RestoreService = (*RestoreMemory)(nil)

// NewRestoreMemory returns a new RestoreMemory instance.
func NewRestoreMemory(backends map[string]BackupBackend, config *model.Config) *RestoreMemory {
	return &RestoreMemory{
		restoreJobs:    NewJobsHolder(),
		restoreService: shared.NewRestore(),
		backends:       backends,
		config:         config,
	}
}

func (r *RestoreMemory) Restore(request *model.RestoreRequestInternal) int {
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

func (r *RestoreMemory) doRestore(request *model.RestoreRequestInternal) bool {
	var success bool
	restoreRunFunc := func() {
		success = r.restoreService.RestoreRun(request)
	}
	out := stdIO.Capture(restoreRunFunc)
	util.LogCaptured(out)
	return success
}

func (r *RestoreMemory) RestoreByTime(request *model.RestoreTimestampRequest) int {
	jobID := r.restoreJobs.newJob()
	go func() {
		backend := r.backends[request.Routine]
		fullBackup, err := r.findLastFullBackup(backend, request)
		if err != nil {
			slog.Error(err.Error(), "JobId", jobID)
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
			slog.Error(err.Error(), "JobId", jobID)
			r.restoreJobs.setFailed(jobID)
			return
		}

		if err = r.restoreIncrementalBackups(incrementalBackups, request); err != nil {
			slog.Error(err.Error(), "JobId", jobID)
			r.restoreJobs.setFailed(jobID)
			return
		}

		r.restoreJobs.setDone(jobID)
	}()
	return jobID
}

func (r *RestoreMemory) findLastFullBackup(backend BackupBackend,
	request *model.RestoreTimestampRequest) (*model.BackupDetails, error) {
	fullBackupList, err := backend.FullBackupList()
	if err != nil {
		slog.Error("cannot read full backup list", "name", request.Routine, "error", err)
		return nil, fmt.Errorf("cannot read full backup list for %s", request.Routine)
	}

	fullBackup := latestFullBackupBeforeTime(fullBackupList, time.UnixMilli(request.Time))
	if fullBackup == nil {
		return nil, fmt.Errorf("no full backup found for %s at %d", request.Routine, request.Time)
	}

	return fullBackup, nil
}

func latestFullBackupBeforeTime(list []model.BackupDetails, time time.Time) *model.BackupDetails {
	var latestFullBackup *model.BackupDetails
	for i := range list {
		b := &list[i]
		if b.LastModified.Before(time) {
			if latestFullBackup == nil || latestFullBackup.LastModified.After(*b.LastModified) {
				latestFullBackup = b
			}
		}
	}
	return latestFullBackup
}

func (r *RestoreMemory) restoreFullBackup(request *model.RestoreTimestampRequest, key *string) bool {
	return r.doRestore(&model.RestoreRequestInternal{
		RestoreRequest: model.RestoreRequest{
			DestinationCuster: request.DestinationCuster,
			SourceStorage:     r.config.Storage[request.Routine],
			Policy:            request.Policy,
		},
		Dir: key})
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

func (r *RestoreMemory) restoreIncrementalBackups(
	incrementalBackups []model.BackupDetails, request *model.RestoreTimestampRequest) error {
	for _, b := range incrementalBackups {
		incrRestoreOK := r.doRestore(&model.RestoreRequestInternal{
			RestoreRequest: model.RestoreRequest{
				DestinationCuster: request.DestinationCuster,
				SourceStorage:     r.config.Storage[request.Routine],
				Policy:            request.Policy,
			},
			File: b.Key,
		})
		if !incrRestoreOK {
			return fmt.Errorf("could not restore incremental backup %s at %s", request.Routine, *b.Key)
		}
	}
	return nil
}

// JobStatus returns the status of the job with the given id.
func (r *RestoreMemory) JobStatus(jobID int) string {
	return r.restoreJobs.getStatus(jobID)
}
