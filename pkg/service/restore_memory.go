package service

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/aerospike/aerospike-client-go/v7"
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
	backends       BackendsHolder
}

var _ RestoreService = (*RestoreMemory)(nil)

// NewRestoreMemory returns a new RestoreMemory instance.
func NewRestoreMemory(backends BackendsHolder, config *model.Config, restoreService shared.Restore) *RestoreMemory {
	return &RestoreMemory{
		restoreJobs:    NewJobsHolder(),
		restoreService: restoreService,
		backends:       backends,
		config:         config,
	}
}

func (r *RestoreMemory) Restore(request *model.RestoreRequestInternal) (int, error) {
	jobID := r.restoreJobs.newJob()
	if err := validateStorageContainsBackup(request.SourceStorage); err != nil {
		return 0, err
	}

	client, aerr := aerospike.NewClientWithPolicyAndHost(
		request.DestinationCuster.ASClientPolicy(),
		request.DestinationCuster.ASClientHosts()...)
	if aerr != nil {
		return 0, fmt.Errorf("failed to connect to aerospike cluster, %w", aerr)
	}

	go func() {
		defer client.Close()
		restoreResult, err := r.restoreService.RestoreRun(client, request)
		if err != nil {
			r.restoreJobs.setFailed(jobID, fmt.Errorf("failed restore operation: %w", err))
			return
		}
		r.restoreJobs.increaseStats(jobID, restoreResult)
		r.restoreJobs.setDone(jobID)
	}()

	return jobID, nil
}

func (r *RestoreMemory) RestoreByTime(request *model.RestoreTimestampRequest) (int, error) {
	reader, found := r.backends.GetReader(request.Routine)
	if !found {
		return 0, fmt.Errorf("backend '%s' not found for restore", request.Routine)
	}
	fullBackups, err := r.findLastFullBackup(reader, request.Time)
	if err != nil {
		return 0, fmt.Errorf("last full backup not found: %v", err)
	}
	jobID := r.restoreJobs.newJob()
	client, aerr := aerospike.NewClientWithPolicyAndHost(
		request.DestinationCuster.ASClientPolicy(),
		request.DestinationCuster.ASClientHosts()...)
	if aerr != nil {
		return 0, fmt.Errorf("failed to connect to aerospike cluster, %w", aerr)
	}
	go r.restoreByTimeSync(client, reader, request, jobID, fullBackups)
	return jobID, nil
}

func (r *RestoreMemory) restoreByTimeSync(
	client *aerospike.Client,
	backend BackupListReader,
	request *model.RestoreTimestampRequest,
	jobID int,
	fullBackups []model.BackupDetails,
) {
	var wg sync.WaitGroup

	for _, nsBackup := range fullBackups {
		wg.Add(1)
		go func(nsBackup model.BackupDetails) {
			defer wg.Done()
			if err := r.restoreNamespace(client, backend, request, jobID, nsBackup); err != nil {
				slog.Error("Failed to restore by timestamp", "routine", request.Routine, "err", err)
				r.restoreJobs.setFailed(jobID, err)
				return
			}
		}(nsBackup)
	}

	wg.Wait()

	r.restoreJobs.setDone(jobID)
	client.Close()
}

func (r *RestoreMemory) restoreNamespace(
	client *aerospike.Client,
	backend BackupListReader,
	request *model.RestoreTimestampRequest,
	jobID int, fullBackup model.BackupDetails,
) error {
	result, err := r.restoreFromPath(client, request, fullBackup.Key)
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
		result, err := r.restoreFromPath(client, request, incrBackup.Key)
		if err != nil {
			return fmt.Errorf("could not restore incremental backup %s: %v", *incrBackup.Key, err)
		}
		r.restoreJobs.increaseStats(jobID, result)
	}
	return nil
}

func (r *RestoreMemory) restoreFromPath(
	client *aerospike.Client,
	request *model.RestoreTimestampRequest,
	backupPath *string,
) (*model.RestoreResult, error) {
	restoreRequest := r.toRestoreRequest(request)
	restoreResult, err := r.restoreService.RestoreRun(
		client,
		&model.RestoreRequestInternal{
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

func (r *RestoreMemory) RetrieveConfiguration(routine string, toTimeMillis int64) ([]byte, error) {
	backend, found := r.backends.GetReader(routine)
	if !found {
		return nil, fmt.Errorf("backend '%s' not found for restore", routine)
	}
	fullBackups, err := r.findLastFullBackup(backend, toTimeMillis)
	if err != nil || len(fullBackups) == 0 {
		return nil, fmt.Errorf("last full backup not found: %v", err)
	}

	// fullBackups has backups for multiple namespaces, but same timestamp, they share same configuration.
	lastFullBackup := fullBackups[0]
	configPath, err := calculateConfigurationBackupPath(*lastFullBackup.Key)
	if err != nil {
		return nil, err
	}
	return backend.ReadClusterConfiguration(configPath)
}

func calculateConfigurationBackupPath(backupKey string) (string, error) {
	_, path, err := util.ParseS3Path(backupKey)
	if err != nil {
		return "", err
	}
	// Move up two directories
	base := filepath.Dir(filepath.Dir(path))
	// Join new directory 'config' with the new base
	return filepath.Join(base, model.ConfigurationBackupDirectory), nil
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
