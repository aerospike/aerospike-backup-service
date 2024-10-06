package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/storage"
	"github.com/aerospike/backup-go"
	"github.com/prometheus/client_golang/prometheus"
)

var errBackendNotFound = errors.New("backend not found")
var errBackupNotFound = errors.New("backup not found")

// dataRestorer implements the RestoreManager interface.
// Stores job information locally within a map.
type dataRestorer struct {
	configRetriever
	config         *model.Config
	restoreJobs    *JobsHolder
	restoreService Restore
	backends       BackendsHolder
	clientManager  ClientManager
}

var _ RestoreManager = (*dataRestorer)(nil)

// NewRestoreManager returns a new dataRestorer instance.
func NewRestoreManager(backends BackendsHolder,
	config *model.Config,
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

func (r *dataRestorer) Restore(request *model.RestoreRequest) (model.RestoreJobID, error) {
	jobID := r.restoreJobs.newJob()
	ctx := context.TODO()
	totalRecords, err := recordsInBackup(ctx, request)
	if err != nil {
		slog.Info("Could not read backup metadata", slog.Any("err", err))
	}

	go func() {
		client, err := r.clientManager.GetClient(request.DestinationCuster)
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
		err = handler.Wait(ctx)
		if err != nil {
			r.restoreJobs.setFailed(jobID, fmt.Errorf("failed restore operation: %w", err))
			return
		}

		r.restoreJobs.setDone(jobID)
	}()

	return jobID, nil
}

func (r *dataRestorer) RestoreByTime(request *model.RestoreTimestampRequest,
) (model.RestoreJobID, error) {
	reader, found := r.backends.GetReader(request.Routine)
	if !found {
		return 0, fmt.Errorf("%w: routine %s", errBackendNotFound, request.Routine)
	}
	fullBackups, err := reader.FindLastFullBackup(request.Time)
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
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	fullBackups []model.BackupDetails,
) {
	client, err := r.clientManager.GetClient(request.DestinationCuster)
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
		go func(nsBackup model.BackupDetails) {
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
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	fullBackup model.BackupDetails,
) error {
	allBackups := []model.BackupDetails{fullBackup}

	// Find incremental backups
	bounds, err := model.NewTimeBounds(&fullBackup.Created, &request.Time)
	if err != nil {
		return err
	}

	incrementalBackups, err := backend.FindIncrementalBackupsForNamespace(ctx, bounds, fullBackup.Namespace)
	if err != nil {
		return fmt.Errorf("could not find incremental backups for namespace %s: %w", fullBackup.Namespace, err)
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

		err = handler.Wait(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *dataRestorer) restoreFromPath(
	ctx context.Context,
	client *backup.Client,
	request *model.RestoreTimestampRequest,
	backupPath string,
) (RestoreHandler, error) {
	restoreRequest := r.toRestoreRequest(request)
	restoreRequest.BackupDataPath = backupPath
	handler, err := r.restoreService.RestoreRun(ctx, client, restoreRequest)
	if err != nil {
		return nil, fmt.Errorf("could not start restore from backup at %s: %w", backupPath, err)
	}

	return handler, nil
}

func (r *dataRestorer) toRestoreRequest(request *model.RestoreTimestampRequest) *model.RestoreRequest {
	routine := r.config.BackupRoutines[request.Routine]
	return model.NewRestoreRequest(
		request.DestinationCuster,
		request.Policy,
		routine.Storage,
		request.SecretAgent,
	)
}

// JobStatus returns the status of the job with the given id.
func (r *dataRestorer) JobStatus(jobID model.RestoreJobID) (*model.RestoreJobStatus, error) {
	return r.restoreJobs.getStatus(jobID)
}

func recordsInBackup(ctx context.Context, request *model.RestoreRequest) (uint64, error) {
	bytes, err := storage.ReadFile(ctx, request.SourceStorage, filepath.Join(request.BackupDataPath, metadataFile))
	if err != nil {
		return 0, err
	}
	metadata, err := model.NewMetadataFromBytes(bytes)
	if err != nil {
		return 0, err
	}
	return metadata.RecordCount, nil
}
