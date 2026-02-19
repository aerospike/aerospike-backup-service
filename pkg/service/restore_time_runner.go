package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
)

type timeRestoreRunner struct {
	restoreJobs    *RestoreJobsHolder
	restoreService restoreexecutor.Restore
	backupReader   BackupReader
	clientManager  aerospike.ClientManager
	routineStorage *collections.LockMap
	preflight      RestorePreflight
}

func newTimeRestoreRunner(
	restoreJobs *RestoreJobsHolder,
	restoreService restoreexecutor.Restore,
	backupReader BackupReader,
	clientManager aerospike.ClientManager,
	routineStorage *collections.LockMap,
	preflight RestorePreflight,
) *timeRestoreRunner {
	return &timeRestoreRunner{
		restoreJobs:    restoreJobs,
		restoreService: restoreService,
		backupReader:   backupReader,
		clientManager:  clientManager,
		routineStorage: routineStorage,
		preflight:      preflight,
	}
}

func (r *timeRestoreRunner) RestoreByTime(
	ctx context.Context, request *model.RestoreTimestampRequest,
) (model.RestoreJobID, error) {
	ctx, cancel := context.WithCancel(ctx)
	jobID := r.restoreJobs.newJob(request.Routine.Name, cancel)
	logger := slog.With(slog.Any("jobId", jobID))
	logger.Info("New restore by time job", slog.Any("request", *request))

	go func() {
		err := r.restoreByTimeSync(ctx, request, jobID, logger)
		r.restoreJobs.finishJob(jobID, err)
	}()

	return jobID, nil
}

// findBackupsToRestore returns list of backups for each namespace, sorted by creation date. First is full backup.
func (r *timeRestoreRunner) findBackupsToRestore(
	ctx context.Context, request *model.RestoreTimestampRequest,
) (map[string][]model.BackupDetails, error) {
	backups, err := r.backupReader.GetBackups(ctx,
		NewFullBackupFilter(request.Routine).
			WithToTime(request.Time).
			Last(),
	)
	// backups contains list of full backups for every namespace.
	// They all have same created time, routine name and storage.
	if err != nil {
		return nil, fmt.Errorf("failed to read last full backup: %w", err)
	}

	if len(backups) == 0 {
		return nil, errors.New("no full backups found")
	}

	// Find incremental backups.
	incrementalBackups, err := r.backupReader.GetBackups(ctx,
		NewIncrementalBackupFilter(request.Routine).
			WithFromTime(backups[0].Created).
			WithToTime(request.Time))
	if err != nil {
		return nil, fmt.Errorf("could not find incremental backups: %w", err)
	}

	backupsByNs := make(map[string][]model.BackupDetails)
	for _, b := range backups {
		backupsByNs[b.Namespace] = append(backupsByNs[b.Namespace], b)
	}

	for _, b := range incrementalBackups {
		backupsByNs[b.Namespace] = append(backupsByNs[b.Namespace], b)
	}

	return backupsByNs, nil
}

func (r *timeRestoreRunner) restoreByTimeSync(
	ctx context.Context,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	logger *slog.Logger,
) error {
	// Lock the routine storage from retention manager for the duration of restore.
	// Restore holds RLock to allow concurrent restores for the same routine.
	routineStorageLock := r.routineStorage.Get(request.Routine.Name)
	routineStorageLock.RLock()
	defer routineStorageLock.RUnlock()

	backupsByNamespace, err := r.findBackupsToRestore(ctx, request)
	if err != nil {
		return err
	}

	client, err := r.clientManager.GetClient(ctx, request.DestinationCluster, logger)
	if err != nil {
		return fmt.Errorf("failed to get client for cluster %s: %w",
			ptr.ValueOrZero(request.DestinationCluster.ClusterLabel), err)
	}
	defer r.clientManager.Close(client)

	if err := r.preflight.ValidateTimeRestore(
		ctx,
		request.DestinationCluster,
		request.Policy,
		client.InfoClient(),
		backupsByNamespace,
		request,
	); err != nil {
		return err
	}

	return r.restoreAllNamespaces(ctx, client, request, jobID, logger, backupsByNamespace)
}

func (r *timeRestoreRunner) restoreAllNamespaces(
	ctx context.Context,
	client aerospike.Client,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	logger *slog.Logger,
	backupsByNamespace map[string][]model.BackupDetails,
) error {
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
						request.Routine.Name, namespace, err))
				errMu.Unlock()
			}
		}(namespace, nsBackup)
	}

	wg.Wait()

	return multiError
}

func (r *timeRestoreRunner) restoreNamespace(
	ctx context.Context,
	client aerospike.Restorer,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	namespace string,
	backups []model.BackupDetails,
	logger *slog.Logger,
) error {
	// Policy is guaranteed non-nil by request validation in the API layer.
	effectivePolicy := *request.Policy // make a thread-safe copy.

	// Restore all backups in order.
	if !request.DisableReordering {
		counter, err := client.InfoClient().GetRecordCount(ctx, namespace, effectivePolicy.SetList)
		if err != nil {
			return fmt.Errorf("could not determine if namespace %s is empty: %w", namespace, err)
		}

		if counter == 0 {
			logger.Info("Use optimized restore because database is empty")
			// If the data is restored to an empty cluster reverse the order using the CREATE_ONLY policy.
			// This way we reduce generation noise and unnecessary load.
			slices.Reverse(backups)

			// old values are not important, because they qualify how to handle existing data in db.
			effectivePolicy.Unique = ptr.Of(true)
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

		// For restore-by-time we can extract compression from metadata.
		effectivePolicy.CompressionPolicy = &model.CompressionPolicy{
			Mode: b.Compression,
		}

		handler, err := r.restoreFromPath(ctx, client, request, b.Key, b.Storage, &effectivePolicy)
		if err != nil {
			return err
		}

		r.restoreJobs.addHandler(jobID, handler)

		if err = handler.Wait(ctx); err != nil {
			return err
		}
		logger.LogAttrs(ctx, slog.LevelInfo, "Finished restoring", logAttrs(handler.GetStats())...)
	}

	return nil
}

func (r *timeRestoreRunner) restoreFromPath(
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
