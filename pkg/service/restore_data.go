package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/pkg/dto"
	"github.com/aerospike/backup-go"
	"github.com/aws/smithy-go/ptr"
	"github.com/prometheus/client_golang/prometheus"
)

var errBackendNotFound = errors.New("backend not found")
var errBackupNotFound = errors.New("backup not found")

// dataRestorer implements the RestoreManager interface.
// Stores job information locally within a map.
type dataRestorer struct {
	configRetriever
	config         *dto.Config
	restoreJobs    *JobsHolder
	restoreService Restore
	backends       BackendsHolder
	clientManager  ClientManager
}

var _ RestoreManager = (*dataRestorer)(nil)

// NewRestoreManager returns a new dataRestorer instance.
func NewRestoreManager(backends BackendsHolder,
	config *dto.Config,
	restoreService Restore,
	clientManager ClientManager,
) RestoreManager {
	return &dataRestorer{
		configRetriever: configRetriever{
			backends,
		},
		restoreJobs:    NewJobsHolder(),
		restoreService: restoreService,
		backends:       backends,
		config:         config,
		clientManager:  clientManager,
	}
}

func (r *dataRestorer) Restore(request *dto.RestoreRequestInternal,
) (dto.RestoreJobID, error) {
	jobID := r.restoreJobs.newJob()
	totalRecords, err := validateStorageContainsBackup(request.SourceStorage)
	if err != nil {
		return 0, err
	}

	ctx := context.TODO()
	go func() {
		client, err := r.clientManager.CreateClient(request.DestinationCuster)
		if err != nil {
			slog.Error("Failed to restore by path",
				slog.Any("cluster", request.DestinationCuster),
				slog.Any("err", err))
			r.restoreJobs.setFailed(jobID, err)
			return
		}
		defer r.clientManager.Close(client)

		handler, err := r.restoreService.RestoreRun(ctx, client, request)
		if err != nil {
			r.restoreJobs.setFailed(jobID, fmt.Errorf("failed to start restore operation: %w", err))
			return
		}
		r.restoreJobs.addTotalRecords(jobID, totalRecords)
		r.restoreJobs.addHandler(jobID, handler)

		// Wait for the restore operation to complete
		err = handler.Wait()
		if err != nil {
			r.restoreJobs.setFailed(jobID, fmt.Errorf("failed restore operation: %w", err))
			return
		}

		r.restoreJobs.setDone(jobID)
	}()

	return jobID, nil
}

func (r *dataRestorer) RestoreByTime(request *dto.RestoreTimestampRequest,
) (dto.RestoreJobID, error) {
	reader, found := r.backends.GetReader(request.Routine)
	if !found {
		return 0, fmt.Errorf("%w: routine %s", errBackendNotFound, request.Routine)
	}
	timestamp := time.UnixMilli(request.Time)
	fullBackups, err := reader.FindLastFullBackup(timestamp)
	if err != nil {
		return 0, fmt.Errorf("restore failed: %w", err)
	}
	jobID := r.restoreJobs.newJob()
	ctx := context.TODO()
	go r.restoreByTimeSync(ctx, reader, request, jobID, fullBackups)

	return jobID, nil
}

func (r *dataRestorer) restoreByTimeSync(
	ctx context.Context,
	backend BackupListReader,
	request *dto.RestoreTimestampRequest,
	jobID dto.RestoreJobID,
	fullBackups []dto.BackupDetails,
) {
	client, err := r.clientManager.CreateClient(request.DestinationCuster)
	if err != nil {
		slog.Error("Failed to restore by timestamp",
			slog.Any("cluster", request.DestinationCuster),
			slog.Any("err", err))
		r.restoreJobs.setFailed(jobID, err)
		return
	}
	defer r.clientManager.Close(client)

	var wg sync.WaitGroup

	multiError := prometheus.MultiError{}
	for _, nsBackup := range fullBackups {
		wg.Add(1)
		go func(nsBackup dto.BackupDetails) {
			defer wg.Done()
			if err := r.restoreNamespace(ctx, client, backend, request, jobID, nsBackup); err != nil {
				multiError.Append(
					fmt.Errorf("failed to restore routine %s, namespace %s by timestamp: %w",
						request.Routine, nsBackup.Namespace, err))
			}
		}(nsBackup)
	}

	wg.Wait()

	err = multiError.MaybeUnwrap()
	if err != nil {
		r.restoreJobs.setFailed(jobID, err)
		return
	}

	r.restoreJobs.setDone(jobID)
}

func (r *dataRestorer) restoreNamespace(
	ctx context.Context,
	client *backup.Client,
	backend BackupListReader,
	request *dto.RestoreTimestampRequest,
	jobID dto.RestoreJobID,
	fullBackup dto.BackupDetails,
) error {
	allBackups := []dto.BackupDetails{fullBackup}

	// Find incremental backups
	bounds, err := dto.NewTimeBounds(&fullBackup.Created, ptr.Time(time.UnixMilli(request.Time)))
	if err != nil {
		return err
	}

	incrementalBackups, err := backend.FindIncrementalBackupsForNamespace(bounds,
		fullBackup.Namespace)
	if err != nil {
		return fmt.Errorf("could not find incremental backups for namespace %s: %w",
			fullBackup.Namespace, err)
	}

	// Append incremental backups to allBackups
	allBackups = append(allBackups, incrementalBackups...)

	for _, b := range allBackups {
		r.restoreJobs.addTotalRecords(jobID, b.RecordCount)
	}

	// Now restore all backups in order
	for _, b := range allBackups {
		handler, err := r.restoreFromPath(ctx, client, request, b.Key)
		if err != nil {
			return err
		}
		r.restoreJobs.addHandler(jobID, handler)

		err = handler.Wait()
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *dataRestorer) restoreFromPath(
	ctx context.Context,
	client *backup.Client,
	request *dto.RestoreTimestampRequest,
	backupPath *string,
) (RestoreHandler, error) {
	restoreRequest := r.toRestoreRequest(request)
	handler, err := r.restoreService.RestoreRun(ctx,
		client,
		&dto.RestoreRequestInternal{
			RestoreRequest: *restoreRequest,
			Dir:            backupPath,
		})
	if err != nil {
		return nil, fmt.Errorf("could not start restore from backup at %s: %w", *backupPath, err)
	}

	return handler, nil
}

func (r *dataRestorer) toRestoreRequest(request *dto.RestoreTimestampRequest) *dto.RestoreRequest {
	routine := r.config.BackupRoutines[request.Routine]
	storage := r.config.Storage[routine.Storage]
	return dto.NewRestoreRequest(
		request.DestinationCuster,
		request.Policy,
		storage,
		request.SecretAgent,
	)
}

// JobStatus returns the status of the job with the given id.
func (r *dataRestorer) JobStatus(jobID dto.RestoreJobID) (*dto.RestoreJobStatus, error) {
	return r.restoreJobs.getStatus(jobID)
}

func validateStorageContainsBackup(storage *dto.Storage) (uint64, error) {
	switch storage.Type {
	case dto.Local:
		return validatePathContainsBackup(*storage.Path)
	case dto.S3:
		return NewS3Context(storage).ValidateStorageContainsBackup()
	}
	return 0, nil
}
