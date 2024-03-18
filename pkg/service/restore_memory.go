package service

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"time"

	"github.com/aerospike/backup/pkg/model"
	"github.com/aerospike/backup/pkg/shared"
	"github.com/aerospike/backup/pkg/stdio"
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
	if err := validateStorageContainsBackup(request.SourceStorage); err != nil {
		return 0, err
	}
	go func() {
		restoreResult, err := r.runRestoreService(request)
		if err != nil {
			r.restoreJobs.setFailed(jobID, fmt.Errorf("failed restore operation: %w", err))
			return
		}
		r.restoreJobs.increaseStats(jobID, restoreResult)
		r.restoreJobs.setDone(jobID)
	}()
	return jobID, nil
}

func (r *RestoreMemory) runRestoreService(request *model.RestoreRequestInternal) (*model.RestoreResult, error) {
	var result *model.RestoreResult
	var err error
	restoreRunFunc := func() {
		request.SourceStorage.SetDefaultProfile()
		result, err = r.restoreService.RestoreRun(request)
	}
	out := stdio.Stderr.Capture(restoreRunFunc)
	util.LogCaptured(out)
	return result, err
}

func (r *RestoreMemory) RestoreByTime(request *model.RestoreTimestampRequest) (int, error) {
	backend, found := r.backends[request.Routine]
	if !found {
		return 0, fmt.Errorf("backend '%s' not found for restore", request.Routine)
	}
	fullBackups, err := r.findLastFullBackup(backend, request.Time)
	if err != nil {
		return 0, fmt.Errorf("last full backup not found: %v", err)
	}
	jobID := r.restoreJobs.newJob()
	go r.restoreByTimeSync(backend, request, jobID, fullBackups)
	return jobID, nil
}

func (r *RestoreMemory) restoreByTimeSync(backend BackupListReader,
	request *model.RestoreTimestampRequest,
	jobID int,
	fullBackups []model.BackupDetails,
) {
	for _, nsBackup := range fullBackups {
		if err := r.restoreNamespace(backend, request, jobID, nsBackup); err != nil {
			slog.Error("Failed to restore by timestamp", "routine", request.Routine, "err", err)
			r.restoreJobs.setFailed(jobID, err)
			return
		}
	}
	r.restoreJobs.setDone(jobID)
}

func (r *RestoreMemory) restoreNamespace(
	backend BackupListReader,
	request *model.RestoreTimestampRequest,
	jobID int, fullBackup model.BackupDetails,
) error {
	result, err := r.restoreFromPath(request, fullBackup.Key)
	if err != nil {
		return fmt.Errorf("could not restore full backup for namespace %s: %v", fullBackup.Namespace, err)
	}
	r.restoreJobs.increaseStats(jobID, result)

	incrementalBackups, err := r.findIncrementalBackupsForNamespace(
		backend, fullBackup.Created.UnixMilli(), request.Time, fullBackup.Namespace)
	if err != nil {
		return fmt.Errorf("could not find incremental backups for namespace %s: %v", fullBackup.Namespace, err)
	}
	slog.Info("Apply incremental backups", "size", len(incrementalBackups))
	for _, incrBackup := range incrementalBackups {
		result, err := r.restoreFromPath(request, incrBackup.Key)
		if err != nil {
			return fmt.Errorf("could not restore incremental backup %s: %v", *incrBackup.Key, err)
		}
		r.restoreJobs.increaseStats(jobID, result)
	}
	return nil
}

func (r *RestoreMemory) restoreFromPath(
	request *model.RestoreTimestampRequest,
	backupPath *string,
) (*model.RestoreResult, error) {
	restoreRequest := r.toRestoreRequest(request)
	restoreResult, err := r.runRestoreService(&model.RestoreRequestInternal{
		RestoreRequest: *restoreRequest,
		Dir:            backupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("could not restore backup at %s: %w", *backupPath, err)
	}

	return restoreResult, nil
}

func (r *RestoreMemory) findLastFullBackup(
	backend BackupListReader,
	toTimeMillis int64,
) ([]model.BackupDetails, error) {
	to, err := model.NewTimeBoundsTo(toTimeMillis)
	if err != nil {
		return nil, err
	}
	fullBackupList, err := backend.FullBackupList(to)
	if err != nil {
		return nil, fmt.Errorf("cannot read full backup list: %v", err)
	}

	fullBackup := latestFullBackupBeforeTime(fullBackupList, time.UnixMilli(toTimeMillis)) // it's a list of namespaces
	if len(fullBackup) == 0 {
		return nil, fmt.Errorf("no full backup found at %d", toTimeMillis)
	}
	return fullBackup, nil
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

func (r *RestoreMemory) findIncrementalBackupsForNamespace(
	backend BackupListReader, from, to int64, namespace string) ([]model.BackupDetails, error) {
	bounds, err := model.NewTimeBounds(&from, &to)
	if err != nil {
		return nil, err
	}
	allIncrementalBackupList, err := backend.IncrementalBackupList(bounds)
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

func (r *RestoreMemory) RestoreConfiguration(routine string, toTimeMillis int64) ([]byte, error) {
	backend, found := r.backends[routine]
	if !found {
		return nil, fmt.Errorf("backend '%s' not found for restore", routine)
	}
	fullBackups, err := r.findLastFullBackup(backend, toTimeMillis)
	if err != nil {
		return nil, fmt.Errorf("last full backup not found: %v", err)
	}

	// fullBackups has backups for multiple namespaces, but same timestamp, they share same configuration.
	lastFullBackup := fullBackups[0]
	configPath := calculateConfigurationBackupPath(*lastFullBackup.Key)
	return backend.ReadClusterConfiguration(configPath)
}

func calculateConfigurationBackupPath(path string) string {
	// Move up two directories
	base := filepath.Dir(filepath.Dir(path))
	// Join new directory 'config' with the new base
	return filepath.Join(base, model.ConfigurationBackupDirectory)
}

func (r *RestoreMemory) toRestoreRequest(request *model.RestoreTimestampRequest) *model.RestoreRequest {
	routine := r.config.BackupRoutines[request.Routine]
	storage := r.config.Storage[routine.Storage]
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

func validateStorageContainsBackup(storage *model.Storage) error {
	switch storage.Type {
	case model.Local:
		return validatePathContainsBackup(*storage.Path)
	case model.S3:
		context, err := NewS3Context(storage)
		if err != nil {
			return err
		}
		return context.validateStorageContainsBackup()
	}
	return nil
}
