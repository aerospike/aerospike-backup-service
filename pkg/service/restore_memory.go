package service

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/aerospike/backup/pkg/util"
)

// RestoreMemory implements the RestoreService interface.
// Stores job information locally within a map.
type RestoreMemory struct {
	config         *model.Config
	restoreJobs    *JobsHolder
	restoreService shared.Restore
	backends       map[string]BackupListReader
}

var _ RestoreService = (*RestoreMemory)(nil)

// NewRestoreMemory returns a new RestoreMemory instance.
func NewRestoreMemory(backends map[string]BackupListReader, config *model.Config) *RestoreMemory {
	return &RestoreMemory{
		restoreJobs:    NewJobsHolder(),
		restoreService: shared.NewRestore(),
		backends:       backends,
		config:         config,
	}
}

func (r *RestoreMemory) Restore(request *model.RestoreRequestInternal) (int, error) {
	jobID := r.restoreJobs.newJob()
	if err := validate(request.Dir, request.SourceStorage); err != nil {
		return 0, err
	}
	go func() {
		restoreResult := r.doRestore(request)
		if restoreResult == nil {
			r.restoreJobs.setFailed(jobID, fmt.Errorf("failed restore operation"))
			return
		}
		r.restoreJobs.increaseStats(jobID, restoreResult)
		r.restoreJobs.setDone(jobID)
	}()
	return jobID, nil
}

func (r *RestoreMemory) doRestore(request *model.RestoreRequestInternal) *model.RestoreResult {
	var result *model.RestoreResult
	restoreRunFunc := func() {
		request.SourceStorage.SetDefaultProfile()
		result = r.restoreService.RestoreRun(request)
	}
	out := stdIO.Capture(restoreRunFunc)
	util.LogCaptured(out)
	return result
}

func (r *RestoreMemory) RestoreByTime(request *model.RestoreTimestampRequest) (int, error) {
	backend, found := r.backends[request.Routine]
	if !found {
		slog.Error("Backend not found for restore", "routine", request.Routine)
		return 0, fmt.Errorf("backend %s not found for restore", request.Routine)
	}

	jobID := r.restoreJobs.newJob()
	go r.restoreByTimeSync(backend, request, jobID)
	return jobID, nil
}

func (r *RestoreMemory) restoreByTimeSync(backend BackupListReader, request *model.RestoreTimestampRequest, jobID int) {
	fullBackups, err := r.findLastFullBackup(backend, request)
	if err != nil {
		slog.Error("Could not find last full backup", "JobId", jobID, "routine", request.Routine,
			"err", err)
		r.restoreJobs.setFailed(jobID, err)
		return
	}
	for _, nsBackup := range fullBackups {
		r.restoreNamespace(backend, request, jobID, nsBackup)
	}

	r.restoreJobs.setDone(jobID)
}

func (r *RestoreMemory) findLastFullBackup(
	backend BackupListReader,
	request *model.RestoreTimestampRequest,
) ([]model.BackupDetails, error) {
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

func (r *RestoreMemory) restoreNamespace(
	backend BackupListReader, request *model.RestoreTimestampRequest, jobID int, fullBackup model.BackupDetails) {
	result, err := r.restore(request, fullBackup.Key)
	if err != nil {
		slog.Error("Could not restore full backup", "JobId", jobID, "routine", request.Routine, "err", err)
		r.restoreJobs.setFailed(jobID, err)
		return
	}
	r.restoreJobs.increaseStats(jobID, result)

	incrementalBackups, err := r.findIncrementalBackupsForNamespace(
		backend, fullBackup.Created.UnixMilli(), request.Time, fullBackup.Namespace)
	if err != nil {
		slog.Error("Could not find incremental backups",
			"JobId", jobID, "routine", request.Routine, "err", err)
		r.restoreJobs.setFailed(jobID, err)
		return
	}
	slog.Info("Apply incremental backups", "size", len(incrementalBackups))
	for _, incrBackup := range incrementalBackups {
		result, err := r.restore(request, incrBackup.Key)
		if err != nil {
			slog.Error("Could not restore incremental backups", "JobId", jobID, "routine", request.Routine,
				"err", err)
			r.restoreJobs.setFailed(jobID, err)
			return
		}
		r.restoreJobs.increaseStats(jobID, result)
	}
}

// latestFullBackupBeforeTime returns list of backups with same creation time, latest before upperBound.
func latestFullBackupBeforeTime(allBackups []model.BackupDetails, upperBound time.Time) []model.BackupDetails {
	var result []model.BackupDetails
	var latestTime time.Time
	for i := range allBackups {
		current := &allBackups[i]
		if current.Created.After(upperBound) {
			continue
		}

		if len(result) == 0 || latestTime.Before(current.Created) {
			latestTime = current.Created
			result = []model.BackupDetails{*current}
		} else if current.Created.Equal(latestTime) {
			result = append(result, *current)
		}
	}
	return result
}

func (r *RestoreMemory) restore(
	request *model.RestoreTimestampRequest,
	key *string,
) (*model.RestoreResult, error) {
	restoreRequest, err := r.toRestoreRequest(request)
	if err != nil {
		return nil, err
	}
	restoreResult := r.doRestore(&model.RestoreRequestInternal{
		RestoreRequest: *restoreRequest,
		Dir:            key,
	})
	if restoreResult == nil {
		return nil, fmt.Errorf("could not restore backup at %s", *key)
	}

	return restoreResult, nil
}

func (r *RestoreMemory) findIncrementalBackupsForNamespace(
	backend BackupListReader, from, to int64, namespace string) ([]model.BackupDetails, error) {
	allIncrementalBackupList, err := backend.IncrementalBackupList(from, to)
	if err != nil {
		return nil, err
	}
	var filteredIncrementalBackups []model.BackupDetails
	for _, b := range allIncrementalBackupList {
		if b.Namespace == namespace {
			filteredIncrementalBackups = append(filteredIncrementalBackups, b)
		}
	}
	// Sort in place
	sort.Slice(filteredIncrementalBackups, func(i, j int) bool {
		return filteredIncrementalBackups[i].Created.Before(filteredIncrementalBackups[j].Created)
	})

	return filteredIncrementalBackups, nil
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
func (r *RestoreMemory) JobStatus(jobID int) (*model.RestoreJobStatus, error) {
	return r.restoreJobs.getStatus(jobID)
}

func validate(path *string, storage *model.Storage) error {
	switch storage.Type {
	case model.Local:
		return validatePathContainsBackup(*path)
	case model.S3:
		return validateStorageContainsBackup(*path, storage)
	}
	return nil
}

func validateStorageContainsBackup(path string, storage *model.Storage) error {
	context, err := NewS3Context(storage)
	if err != nil {
		return err
	}
	s3prefix := s3Protocol + context.bucket
	files, err := context.listFiles(strings.TrimPrefix(path, s3prefix))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("given path %s not exist", path)
	}
	for _, file := range files {
		if strings.HasSuffix(*file.Key, ".asb") {
			return nil
		}
	}
	return fmt.Errorf("no backup files found in %s", path)
}

func validatePathContainsBackup(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return err
	}

	absFiles, err := filepath.Glob(filepath.Join(path, "*.asb"))
	if err != nil {
		return err
	}
	if len(absFiles) == 0 {
		return fmt.Errorf("no backup files found in %s", path)
	}
	return nil
}
