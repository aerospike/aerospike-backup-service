package service

import (
	"errors"
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
		request.SourceStorage.SetDefaultProfile()
		success = r.restoreService.RestoreRun(request)
	}
	out := stdIO.Capture(restoreRunFunc)
	util.LogCaptured(out)
	return success
}

func (r *RestoreMemory) RestoreByTime(request *model.RestoreTimestampRequest) int {
	jobID := r.restoreJobs.newJob()
	go func() {
		backend, found := r.backends[request.Routine]
		if !found {
			slog.Error("Backend not found for restore", "JobId", jobID, "routine", request.Routine)
			r.restoreJobs.setFailed(jobID)
			return
		}

		fullBackup, err := r.findLastFullBackup(backend, request)
		if err != nil {
			slog.Error("Could not find last full backup", "JobId", jobID, "routine", request.Routine,
				"err", err)
			r.restoreJobs.setFailed(jobID)
			return
		}

		if err = r.restoreFullBackup(request, fullBackup.Key); err != nil {
			slog.Error("Could not restore full backup", "JobId", jobID, "routine", request.Routine,
				"err", err)
			r.restoreJobs.setFailed(jobID)
			return
		}

		incrementalBackups, err := r.findIncrementalBackups(backend, *fullBackup.LastModified)
		if err != nil {
			slog.Error("Could not find incremental backups", "JobId", jobID, "routine", request.Routine,
				"err", err)
			r.restoreJobs.setFailed(jobID)
			return
		}

		if err = r.restoreIncrementalBackups(request, incrementalBackups); err != nil {
			slog.Error("Could not restore incremental backups", "JobId", jobID, "routine", request.Routine,
				"err", err)
			r.restoreJobs.setFailed(jobID)
			return
		}

		r.restoreJobs.setDone(jobID)
	}()
	return jobID
}

func (r *RestoreMemory) findLastFullBackup(
	backend BackupBackend,
	request *model.RestoreTimestampRequest,
) (*model.BackupDetails, error) {
	fullBackupList, err := backend.FullBackupList(0, request.Time)
	if err != nil {
		return nil, fmt.Errorf("cannot read full backup list")
	}

	fullBackup := latestFullBackupBeforeTime(fullBackupList, time.UnixMilli(request.Time))
	if fullBackup == nil {
		return nil, fmt.Errorf("no full backup found at %d", request.Time)
	}

	return fullBackup, nil
}

func latestFullBackupBeforeTime(list []model.BackupDetails, time time.Time) *model.BackupDetails {
	var result *model.BackupDetails
	for i := range list {
		current := &list[i]
		if current.LastModified.After(time) {
			continue
		}
		if result == nil || result.LastModified.Before(*current.LastModified) {
			result = current
		}
	}
	return result
}

func (r *RestoreMemory) restoreFullBackup(
	request *model.RestoreTimestampRequest,
	key *string,
) error {
	restoreRequest, err := r.toRestoreRequest(request)
	if err != nil {
		return err
	}
	fullRestoreOK := r.doRestore(&model.RestoreRequestInternal{
		RestoreRequest: *restoreRequest,
		Dir:            key,
	})
	if !fullRestoreOK {
		return fmt.Errorf("could not restore full backup at %s", *key)
	}

	return nil
}

func (r *RestoreMemory) findIncrementalBackups(
	backend BackupBackend,
	since time.Time,
) ([]model.BackupDetails, error) {
	allIncrementalBackupList, err := backend.IncrementalBackupList()
	if err != nil {
		return nil, err
	}
	var filteredIncrementalBackups []model.BackupDetails
	for _, b := range allIncrementalBackupList {
		if b.LastModified.After(since) {
			filteredIncrementalBackups = append(filteredIncrementalBackups, b)
		}
	}
	return filteredIncrementalBackups, nil
}

func (r *RestoreMemory) restoreIncrementalBackups(
	request *model.RestoreTimestampRequest,
	incrementalBackups []model.BackupDetails,
) error {
	for _, b := range incrementalBackups {
		restoreRequest, err := r.toRestoreRequest(request)
		if err != nil {
			return err
		}
		incrRestoreOK := r.doRestore(&model.RestoreRequestInternal{
			RestoreRequest: *restoreRequest,
			File:           b.Key,
		})
		if !incrRestoreOK {
			return fmt.Errorf("could not restore incremental backup at %s", *b.Key)
		}
	}
	return nil
}

func (r *RestoreMemory) toRestoreRequest(request *model.RestoreTimestampRequest) (*model.RestoreRequest, error) {
	routine, found := r.config.BackupRoutines[request.Routine]
	if !found {
		return nil, errors.New("routine not found")
	}
	storage, found := r.config.Storage[routine.Storage]
	if !found {
		return nil, errors.New("storage not found")
	}
	return model.NewRestoreRequest(
		request.DestinationCuster,
		request.Policy,
		storage,
		request.SecretAgent,
	)
}

// JobStatus returns the status of the job with the given id.
func (r *RestoreMemory) JobStatus(jobID int) string {
	return r.restoreJobs.getStatus(jobID)
}
