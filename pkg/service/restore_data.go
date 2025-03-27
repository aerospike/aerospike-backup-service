package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/aerospike/backup-go"
)

type ErrJobNotFound struct {
	JobID model.RestoreJobID
}

func (e *ErrJobNotFound) Error() string {
	return fmt.Sprintf("restore job with ID %d not found", e.JobID)
}

func NewErrJobNotFound(id model.RestoreJobID) *ErrJobNotFound {
	return &ErrJobNotFound{id}
}

// dataRestorer implements the RestoreManager interface.
// Stores job information locally within a map.
type dataRestorer struct {
	restoreJobs    *RestoreJobsHolder
	restoreService restoreexecutor.Restore
	backupReader   BackupReader
	clientManager  aerospike.ClientManager
	nsValidator    aerospike.NamespaceValidator
}

var _ RestoreManager = (*dataRestorer)(nil)

// NewRestoreManager returns a new dataRestorer instance.
func NewRestoreManager(
	restoreService restoreexecutor.Restore,
	clientManager aerospike.ClientManager,
	restoreJobs *RestoreJobsHolder,
	nsValidator aerospike.NamespaceValidator,
	backupReader BackupReader,
) RestoreManager {
	return &dataRestorer{
		restoreJobs:    restoreJobs,
		restoreService: restoreService,
		backupReader:   backupReader,
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

		if err := r.validateDestinationNamespace(request); err != nil {
			r.restoreJobs.finishJob(jobID, err)
			return
		}

		handler, err := r.restoreService.Run(ctx, client, request)
		if err != nil {
			r.restoreJobs.finishJob(jobID, fmt.Errorf("failed to start restore operation: %w", err))
			return
		}
		r.restoreJobs.addTotalRecords(jobID, totalRecords)
		ctx, cancel := context.WithCancel(ctx)
		r.restoreJobs.addHandler(jobID, restoreexecutor.NewRestoreHandlerWithCancel(handler, cancel))

		// Wait for the restore operation to complete
		err = handler.Wait(ctx)
		r.restoreJobs.finishJob(jobID, err)
	}()

	return jobID, nil
}

// validateDestinationNamespace checks if destination cluster contains namespace from restore request (if it is set).
func (r *dataRestorer) validateDestinationNamespace(request *model.RestoreRequest) error {
	if request.Policy.Namespace != nil {
		missingNamespaces := r.nsValidator.MissingNamespaces(
			request.DestinationCluster, []string{*request.Policy.Namespace.Destination})
		if len(missingNamespaces) > 0 {
			// it can be only 1 missing ns.
			err := fmt.Errorf("destination cluster does not have namespace %s", missingNamespaces[0])
			slog.Error("Failed to restore by path",
				slog.Any("cluster label", request.DestinationCluster.ClusterLabel),
				slog.Any("err", err))
			return err
		}
	}

	return nil
}

func (r *dataRestorer) RestoreByTime(
	ctx context.Context, request *model.RestoreTimestampRequest,
) (model.RestoreJobID, error) {
	fullBackupsByNamespace, err := r.findBackupsToRestore(ctx, request)
	if err != nil {
		return 0, err
	}

	jobID := r.restoreJobs.newJob(request.RoutineName)
	go r.restoreByTimeSync(ctx, request, jobID, fullBackupsByNamespace)

	return jobID, nil
}

// findBackupsToRestore returns list of backups for each namespace, sorted by creation date. First is full backup.
func (r *dataRestorer) findBackupsToRestore(
	ctx context.Context, request *model.RestoreTimestampRequest,
) (map[string][]model.BackupDetails, error) {
	backups, err := r.backupReader.GetBackups(ctx,
		NewFullBackupFilter(request.RoutineName).
			WithToTime(request.Time).
			Last(),
	)
	// backups contains list of full backups for every namespace.
	// They all have same created time, routine name and storage.
	if err != nil {
		return nil, fmt.Errorf("failed to read last full backup: %w", err)
	}

	if len(backups) == 0 {
		return nil, fmt.Errorf("no full backups found")
	}

	// Find incremental backups.
	incrementalBackups, err := r.backupReader.GetBackups(ctx,
		NewIncrementalBackupFilter(request.RoutineName).
			WithFromTime(backups[0].Created).
			WithToTime(request.Time))
	if err != nil {
		return nil, fmt.Errorf("could not find incremental backups: %w", err)
	}

	var backupsByNs = make(map[string][]model.BackupDetails)
	for _, b := range backups {
		backupsByNs[b.Namespace] = append(backupsByNs[b.Namespace], b)
	}

	for _, b := range incrementalBackups {
		backupsByNs[b.Namespace] = append(backupsByNs[b.Namespace], b)
	}

	return backupsByNs, nil
}

func (r *dataRestorer) restoreByTimeSync(
	ctx context.Context,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	fullBackupsByNamespace map[string][]model.BackupDetails,
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
	for namespace, nsBackup := range fullBackupsByNamespace {
		wg.Add(1)
		go func(namespace string, nsBackup []model.BackupDetails) {
			defer wg.Done()
			if err := r.restoreNamespace(ctx, client, request, jobID, namespace, nsBackup); err != nil {
				multiError = errors.Join(multiError,
					fmt.Errorf("failed to restore routine %s, namespace %s by timestamp: %w",
						request.RoutineName, namespace, err))
			}
		}(namespace, nsBackup)
	}

	wg.Wait()

	r.restoreJobs.finishJob(jobID, multiError)
}

func (r *dataRestorer) restoreNamespace(
	ctx context.Context,
	client *backup.Client,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	namespace string,
	backups []model.BackupDetails,
) error {
	// Now restore all backups in order
	dbEmpty, err := r.nsValidator.IsEmpty(client.AerospikeClient(), namespace, request.Policy.SetList)
	if err != nil {
		return fmt.Errorf("could not determine if namespace %s is empty: %w", namespace, err)
	}

	if dbEmpty && !request.DisableReordering {
		// If the data is restored to an empty cluster reverse the order using the CREATE_ONLY policy.
		// This way we reduce generation noise and unnecessary load.
		slices.Reverse(backups)

		// old values are not important, because they qualify how to handle existing data in db.
		request.Policy.Unique = util.Ptr(true)
		request.Policy.Replace = nil
	}

	for _, b := range backups {
		r.restoreJobs.addTotalRecords(jobID, b.RecordCount)
	}

	for _, b := range backups {
		if b.FileCount == 0 { // skip empty namespaces
			continue
		}

		handler, err := r.restoreFromPath(ctx, client, request, b.Key, b.Storage)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(ctx)
		r.restoreJobs.addHandler(jobID, restoreexecutor.NewRestoreHandlerWithCancel(handler, cancel))

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
	storage model.Storage,
) (restoreexecutor.RestoreHandler, error) {
	restoreRequest := model.NewRestoreRequest(
		request.DestinationCluster,
		request.Policy,
		storage,
		request.SecretAgent,
		backupPath,
	)
	handler, err := r.restoreService.Run(ctx, client, restoreRequest)
	if err != nil {
		return nil, fmt.Errorf("could not start restore from backup at %s: %w", backupPath, err)
	}

	return handler, nil
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

// CancelRestore cancels an ongoing restore.
func (r *dataRestorer) CancelRestore(jobID model.RestoreJobID) error {
	job, err := r.restoreJobs.getJob(jobID)
	if err != nil {
		return err
	}
	slog.Debug("Canceling restore job", slog.Any("job ID", jobID))
	for _, h := range job.handlers {
		h.Cancel()
	}

	return nil
}
