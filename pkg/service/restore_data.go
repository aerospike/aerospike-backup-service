package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/service/storage"
	"github.com/aerospike/backup-go"
)

var errBackendNotFound = errors.New("backend not found")
var errBackupNotFound = errors.New("backup not found")

type ErrJobNotFound struct {
	JobID model.RestoreJobID
}

func (e *ErrJobNotFound) Error() string {
	return fmt.Sprintf("restore job with ID %d not found", e.JobID)
}

// dataRestorer implements the RestoreManager interface.
// Stores job information locally within a map.
type dataRestorer struct {
	configRetriever
	config         *model.Config
	restoreJobs    *RestoreJobsHolder
	restoreService Restore
	backends       BackendsHolder
	clientManager  aerospike.ClientManager
	nsValidator    aerospike.NamespaceValidator
}

var _ RestoreManager = (*dataRestorer)(nil)

// NewRestoreManager returns a new dataRestorer instance.
func NewRestoreManager(backends BackendsHolder,
	config *model.Config,
	restoreService Restore,
	clientManager aerospike.ClientManager,
	restoreJobs *RestoreJobsHolder,
	nsValidator aerospike.NamespaceValidator,
) RestoreManager {
	return &dataRestorer{
		configRetriever: configRetriever{
			backends,
		},
		restoreJobs:    restoreJobs,
		restoreService: restoreService,
		backends:       backends,
		config:         config,
		clientManager:  clientManager,
		nsValidator:    nsValidator,
	}
}

func (r *dataRestorer) Restore(request *model.RestoreRequest) (model.RestoreJobID, error) {
	ctx := context.TODO()
	totalRecords, err := recordsInBackup(ctx, request)
	if err != nil {
		slog.Info("Could not read backup metadata", slog.Any("err", err))
	}

	jobID := r.restoreJobs.newJob(request.BackupDataPath)
	go func() {
		client, err := r.clientManager.GetClient(request.DestinationCluster)
		if err != nil {
			slog.Error("Failed to restore by path",
				slog.Any("cluster", request.DestinationCluster.ClusterLabel),
				slog.Any("err", err))
			r.restoreJobs.finishJob(jobID, err)
			return
		}
		defer r.clientManager.Close(client)

		if r.validateDestinationNamespace(request, jobID) {
			return
		}

		handler, err := r.restoreService.Run(ctx, client, request)
		if err != nil {
			r.restoreJobs.finishJob(jobID, fmt.Errorf("failed to start restore operation: %w", err))
			return
		}
		r.restoreJobs.addTotalRecords(jobID, totalRecords)
		ctx, cancel := context.WithCancel(ctx)
		r.restoreJobs.addJob(jobID, &RestoreHandlerWithCancel{
			RestoreHandler: handler,
			cancel:         cancel,
		})

		// Wait for the restore operation to complete
		err = handler.Wait(ctx)
		r.restoreJobs.finishJob(jobID, err)
	}()

	return jobID, nil
}

// validateDestinationNamespace checks if destination cluster contains namespace from restore request (if it is set).
func (r *dataRestorer) validateDestinationNamespace(request *model.RestoreRequest, jobID model.RestoreJobID) bool {
	if request.Policy.Namespace != nil {
		missingNamespaces := r.nsValidator.MissingNamespaces(
			request.DestinationCluster, []string{*request.Policy.Namespace.Destination})
		if len(missingNamespaces) > 0 {
			// it can be only 1 missing ns.
			err := fmt.Errorf("destination cluster does not have namespace %s", missingNamespaces[0])
			slog.Error("Failed to restore by path",
				slog.Any("cluster label", request.DestinationCluster.ClusterLabel),
				slog.Any("err", err))
			r.restoreJobs.finishJob(jobID, err)
			return true
		}
	}

	return false
}

func (r *dataRestorer) RestoreByTime(request *model.RestoreTimestampRequest,
) (model.RestoreJobID, error) {
	reader, found := r.backends.GetReader(request.RoutineName)
	if !found {
		return 0, fmt.Errorf("%w: routine %s", errBackendNotFound, request.RoutineName)
	}
	fullBackups, err := reader.FindLastFullBackup(request.Time)
	if err != nil {
		return 0, fmt.Errorf("restore failed: %w", err)
	}
	jobID := r.restoreJobs.newJob(request.RoutineName)
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
	client, err := r.clientManager.GetClient(request.DestinationCluster)
	if err != nil {
		slog.Error("Failed to restore by timestamp",
			slog.Any("cluster", request.DestinationCluster.ClusterLabel),
			slog.Any("err", err))
		r.restoreJobs.finishJob(jobID, err)
		return
	}
	defer r.clientManager.Close(client)

	var wg sync.WaitGroup
	var multiError error
	for _, nsBackup := range fullBackups {
		wg.Add(1)
		go func(nsBackup model.BackupDetails) {
			defer wg.Done()
			if err := r.restoreNamespace(ctx, client, backend, request, jobID, nsBackup); err != nil {
				multiError = errors.Join(multiError,
					fmt.Errorf("failed to restore routine %s, namespace %s by timestamp: %w",
						request.RoutineName, nsBackup.Namespace, err))
			}
		}(nsBackup)
	}

	wg.Wait()

	r.restoreJobs.finishJob(jobID, multiError)
}

func (r *dataRestorer) restoreNamespace(
	ctx context.Context,
	client *backup.Client,
	backend BackupListReader,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	fullBackup model.BackupDetails,
) error {
	// Find incremental backups
	bounds, err := model.NewTimeBounds(&fullBackup.Created, &request.Time)
	if err != nil {
		return err
	}

	incrementalBackups, err := backend.FindIncrementalBackupsForNamespace(ctx, bounds, fullBackup.Namespace)
	if err != nil {
		return fmt.Errorf("could not find incremental backups for namespace %s: %w", fullBackup.Namespace, err)
	}

	// Now restore all backups in order
	allBackups := append([]model.BackupDetails{fullBackup}, incrementalBackups...)
	for _, b := range allBackups {
		if b.FileCount == 0 { // skip empty namespaces
			continue
		}

		handler, err := r.restoreFromPath(ctx, client, request, b.Key)
		if err != nil {
			return err
		}

		r.restoreJobs.addTotalRecords(jobID, b.RecordCount)
		ctx, cancel := context.WithCancel(ctx)
		r.restoreJobs.addJob(jobID, &RestoreHandlerWithCancel{
			RestoreHandler: handler,
			cancel:         cancel,
		})

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
	handler, err := r.restoreService.Run(ctx, client, restoreRequest)
	if err != nil {
		return nil, fmt.Errorf("could not start restore from backup at %s: %w", backupPath, err)
	}

	return handler, nil
}

func (r *dataRestorer) toRestoreRequest(request *model.RestoreTimestampRequest) *model.RestoreRequest {
	routine := r.config.BackupRoutines[request.RoutineName]
	return model.NewRestoreRequest(
		request.DestinationCluster,
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

func (r *dataRestorer) CancelRestore(jobID model.RestoreJobID) error {
	job, err := r.restoreJobs.getJob(jobID)
	if err != nil {
		return err
	}
	for _, h := range job.handlers {
		h.Cancel()
	}

	return nil
}
