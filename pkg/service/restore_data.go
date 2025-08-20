package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
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
	routineStorage *util.LockMap
}

var _ RestoreManager = (*dataRestorer)(nil)

// NewRestoreManager returns a new dataRestorer instance.
func NewRestoreManager(
	restoreService restoreexecutor.Restore,
	clientManager aerospike.ClientManager,
	restoreJobs *RestoreJobsHolder,
	backupReader BackupReader,
	routineStorage *util.LockMap,
) RestoreManager {
	return &dataRestorer{
		restoreJobs:    restoreJobs,
		restoreService: restoreService,
		backupReader:   backupReader,
		clientManager:  clientManager,
		routineStorage: routineStorage,
	}
}

func (r *dataRestorer) Restore(ctx context.Context, request *model.RestoreRequest) (model.RestoreJobID, error) {
	// Create a cancellable context for this specific job.
	ctx, cancel := context.WithCancel(ctx)

	jobID := r.restoreJobs.newJob(request.BackupDataPath, cancel)
	logger := slog.With(slog.Any("jobId", jobID))
	logger.Info("New restore job", slog.Any("request", *request))
	go func() {
		err := r.executeRestore(ctx, request, jobID, logger)
		if err != nil { // if some of restore sub-operations failed, we need to cancel the rest.
			cancel()
		}
		r.restoreJobs.finishJob(jobID, err)
	}()

	return jobID, nil
}

func (r *dataRestorer) executeRestore(
	ctx context.Context,
	request *model.RestoreRequest,
	jobID model.RestoreJobID,
	logger *slog.Logger,
) error {
	client, err := r.clientManager.GetClient(request.DestinationCluster)
	if err != nil {
		return err
	}
	defer r.clientManager.Close(client)

	if err := r.validateDestinationNamespace(request, client.InfoClient()); err != nil {
		return err
	}

	// Lock the routine storage from retention manager for the duration of restore.
	routineName, _, _ := strings.Cut(request.BackupDataPath, "/") // when restore by path routine is first segment
	if routineName == "" {
		routineName = request.BackupDataPath // fallback to full path
	}

	routineStorageLock := r.routineStorage.Get(routineName)
	routineStorageLock.RLock()
	defer routineStorageLock.RUnlock()

	backups, err := r.backupReader.GetBackups(ctx, NewPathFilter(request.BackupDataPath, request.SourceStorage))
	if err != nil {
		return fmt.Errorf("failed to read backups: %w", err)
	}

	if len(backups) > 0 && r.allBackupsEmpty(backups) {
		// edge case: backups exist but are empty — nothing to restore.
		// If no backups found, we still attempt restore, as CLI-created files may exist without metadata.
		r.restoreJobs.finishJob(jobID, nil)
		logger.Info("Empty backup found, nothing to restore")
		return nil
	}

	if err := r.validateBackupsCreatedAtTheSameTime(backups); err != nil {
		return err
	}

	handler, err := r.restoreService.Run(ctx, client, request)
	if err != nil {
		return fmt.Errorf("failed to start restore operation: %w", err)
	}
	r.restoreJobs.addTotalRecords(jobID, r.recordsInBackup(backups))
	r.restoreJobs.addHandler(jobID, handler)

	return handler.Wait(ctx)
}

func (r *dataRestorer) validateBackupsCreatedAtTheSameTime(backups []model.BackupDetails) error {
	for _, b := range backups {
		if b.Created != backups[0].Created {
			return fmt.Errorf("backups from different times were found: %s and %s",
				b.Created.String(), backups[0].Created.String())
		}
	}

	return nil
}

func (r *dataRestorer) allBackupsEmpty(backups []model.BackupDetails) bool {
	for _, b := range backups {
		if b.FileCount > 0 {
			return false
		}
	}

	return true
}

func (r *dataRestorer) recordsInBackup(backups []model.BackupDetails) uint64 {
	var records uint64
	for _, b := range backups {
		records += b.RecordCount
	}
	return records
}

// validateDestinationNamespace checks if destination cluster contains namespace from restore request (if it is set).
func (r *dataRestorer) validateDestinationNamespace(request *model.RestoreRequest, infoGetter backup.InfoGetter) error {
	if request.Policy.Namespace != nil {
		destinationNS := *request.Policy.Namespace.Destination
		namespaces, err := infoGetter.GetNamespacesList()
		if err != nil {
			return fmt.Errorf("failed to get namespaces from destination cluster: %w", err)
		}
		if !slices.Contains(namespaces, destinationNS) {
			return fmt.Errorf("destination cluster does not have required namespace: %s", destinationNS)
		}
	}

	return nil
}

func (r *dataRestorer) RestoreByTime(
	ctx context.Context, request *model.RestoreTimestampRequest,
) (model.RestoreJobID, error) {
	ctx, cancel := context.WithCancel(ctx)
	jobID := r.restoreJobs.newJob(request.RoutineName, cancel)
	logger := slog.With(slog.Any("jobId", jobID))
	logger.Info("New restore by time job", slog.Any("request", *request))

	go func() {
		err := r.restoreByTimeSync(ctx, request, jobID, logger)
		r.restoreJobs.finishJob(jobID, err)
	}()

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
	logger *slog.Logger,
) error {
	// Lock the routine storage from retention manager for the duration of restore.
	routineStorageLock := r.routineStorage.Get(request.RoutineName)
	routineStorageLock.RLock()
	defer routineStorageLock.RUnlock()

	backupsByNamespace, err := r.findBackupsToRestore(ctx, request)
	if err != nil {
		return err
	}

	client, err := r.clientManager.GetClient(request.DestinationCluster)
	if err != nil {
		return fmt.Errorf("failed to get client for cluster %s: %w",
			util.ValueOrZero(request.DestinationCluster.ClusterLabel), err)
	}
	defer r.clientManager.Close(client)

	var (
		wg         sync.WaitGroup
		multiError error
		errMu      sync.Mutex
	)

	// Run namespace restores concurrently and collect errors safely.
	for namespace, nsBackup := range backupsByNamespace {
		wg.Add(1)
		go func(namespace string, nsBackup []model.BackupDetails) {
			defer wg.Done()

			nsLogger := logger.With(slog.String("namespace", namespace))
			err := r.restoreNamespace(ctx, client, request, jobID, namespace, nsBackup, nsLogger)
			if err != nil {
				errMu.Lock()
				multiError = errors.Join(multiError,
					fmt.Errorf("failed to restore routine %s, namespace %s by timestamp: %w",
						request.RoutineName, namespace, err))
				errMu.Unlock()
			}
		}(namespace, nsBackup)
	}

	wg.Wait()

	return multiError
}

func (r *dataRestorer) restoreNamespace(
	ctx context.Context,
	client aerospike.Restorer,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	namespace string,
	backups []model.BackupDetails,
	logger *slog.Logger,
) error {
	effectivePolicy := *request.Policy // make a thread safe copy.

	// Now restore all backups in order
	if !request.DisableReordering {
		counter, err := client.InfoClient().GetRecordCount(namespace, request.Policy.SetList)
		if err != nil {
			return fmt.Errorf("could not determine if namespace %s is empty: %w", namespace, err)
		}

		if counter == 0 {
			logger.Info("Use optimized restore because database is empty")
			// If the data is restored to an empty cluster reverse the order using the CREATE_ONLY policy.
			// This way we reduce generation noise and unnecessary load.
			slices.Reverse(backups)

			// old values are not important, because they qualify how to handle existing data in db.
			effectivePolicy.Unique = util.Ptr(true)
			effectivePolicy.Replace = nil
		}
	}

	for _, b := range backups {
		r.restoreJobs.addTotalRecords(jobID, b.RecordCount)
	}

	for i, b := range backups {
		logger.Info("Start restoring",
			slog.String("step", fmt.Sprintf("%d/%d", i+1, len(backups))),
			slog.Any("backup", b))
		if b.FileCount == 0 { // skip empty namespaces
			continue
		}

		handler, err := r.restoreFromPath(ctx, client, request, b.Key, b.Storage, &effectivePolicy)
		if err != nil {
			return err
		}

		r.restoreJobs.addHandler(jobID, handler)

		err = handler.Wait(ctx)
		if err != nil {
			return err
		}
		logger.LogAttrs(ctx, slog.LevelInfo, "Finished restoring", logAttrs(handler.GetStats())...)
	}

	return nil
}
func logAttrs(s *models.RestoreStats) []slog.Attr {
	return []slog.Attr{
		slog.Int64("RecordsInserted", int64(s.GetRecordsInserted())),
		slog.Int64("RecordsExpired", int64(s.GetRecordsExpired())),
		slog.Int64("RecordsSkipped", int64(s.GetRecordsSkipped())),
		slog.Int64("RecordsIgnored", int64(s.GetRecordsIgnored())),
		slog.Int64("RecordsFresher", int64(s.GetRecordsFresher())),
		slog.Int64("RecordsExisted", int64(s.GetRecordsExisted())),
		slog.Int64("TotalBytesRead", int64(s.TotalBytesRead.Load())),
		slog.Int64("ErrorsInDoubt", int64(s.GetErrorsInDoubt())),
		slog.Int64("ReadRecords", int64(s.ReadRecords.Load())),
		slog.Int("SecondaryIndexes", int(s.GetSIndexes())),
		slog.Int("UDFs", int(s.GetUDFs())),
		slog.Int64("BytesWritten", int64(s.BytesWritten.Load())),
		slog.Duration("Duration", s.GetDuration()),
	}
}
func (r *dataRestorer) restoreFromPath(
	ctx context.Context,
	client aerospike.Restorer,
	request *model.RestoreTimestampRequest,
	backupPath string,
	storage model.Storage,
	policy *model.RestorePolicy,
) (restoreexecutor.RestoreHandler, error) {
	restoreRequest := model.NewRestoreRequest(
		request.DestinationCluster,
		policy,
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

// CancelRestore cancels an ongoing restore.
func (r *dataRestorer) CancelRestore(jobID model.RestoreJobID) error {
	job, err := r.restoreJobs.getJob(jobID)
	if err != nil {
		return err
	}

	job.cancel()

	return nil
}

// GetFilteredJobs returns all jobs matching the given filters as a map of jobId -> RestoreJobStatus.
func (r *dataRestorer) GetFilteredJobs(
	timeBounds model.TimeBounds,
	statusFilter model.StatusFilter,
) map[model.RestoreJobID]*model.RestoreJobStatus {
	results := make(map[model.RestoreJobID]*model.RestoreJobStatus)

	r.restoreJobs.Iterate(func(id model.RestoreJobID, job *restoreJob) {
		if !timeBounds.Contains(job.started) {
			return
		}

		if !statusFilter.Matches(job.status) {
			return
		}

		results[id] = RestoreJobStatus(job)
	})

	return results
}
