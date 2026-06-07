package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
)

type timeRestoreRunner struct {
	restoreJobs    *RestoreJobsHolder
	restoreService restoreexecutor.Restore
	backupReader   BackupReader
	clientManager  aerospike.ClientManager
	routineStorage *collections.LockMap
	validator      RestoreValidator
}

func newTimeRestoreRunner(
	restoreJobs *RestoreJobsHolder,
	restoreService restoreexecutor.Restore,
	backupReader BackupReader,
	clientManager aerospike.ClientManager,
	routineStorage *collections.LockMap,
	validator RestoreValidator,
) *timeRestoreRunner {
	return &timeRestoreRunner{
		restoreJobs:    restoreJobs,
		restoreService: restoreService,
		backupReader:   backupReader,
		clientManager:  clientManager,
		routineStorage: routineStorage,
		validator:      validator,
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
		r.restoreJobs.finishJob(jobID, err, logger)
	}()

	return jobID, nil
}

// findBackupsToRestore returns list of backups for each namespace, sorted by creation date. First is full backup.
func (r *timeRestoreRunner) findBackupsToRestore(
	ctx context.Context, request *model.RestoreTimestampRequest,
) (map[string][]model.BackupDetails, error) {
	fullBackups, err := r.backupReader.GetBackups(ctx, // find all full backups completed by given restore request time.
		NewFullBackupFilter(&request.Routine).
			WithToTime(request.Time),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read full backups: %w", err)
	}

	if len(fullBackups) == 0 {
		return nil, errors.New("no full backups found")
	}

	// map namespace -> latest full backup for this ns
	latestFullByNamespace := latestBackupsByNamespace(fullBackups)
	// earliest full backup among all
	earliestSelectedFull := earliestSelectedFullBackup(latestFullByNamespace)

	// Strategy:
	// 1) choose the latest full backup per namespace,
	// 2) read incrementals once from the earliest of those selected full backups,
	// 3) then filter incrementals per namespace against that namespace's selected full.
	// This keeps storage reads batched while still producing correct per-namespace chains.
	incrementalBackups, err := r.backupReader.GetBackups(ctx,
		NewIncrementalBackupFilter(&request.Routine).
			WithFromTime(earliestSelectedFull.Created). // lower bound for a single batched incremental scan
			WithToTime(request.Time))
	if err != nil {
		return nil, fmt.Errorf("failed to find incremental backups: %w", err)
	}

	incrementalsByNamespace := splitByNamespace(incrementalBackups)

	return buildRestoreChainsByNamespace(latestFullByNamespace, incrementalsByNamespace), nil
}

// latestBackupsByNamespace picks the newest backup per namespace from the provided list.
func latestBackupsByNamespace(backups []model.BackupDetails) map[string]model.BackupDetails {
	latestByNamespace := make(map[string]model.BackupDetails, len(backups))
	for _, b := range backups {
		current, exists := latestByNamespace[b.Namespace]
		if !exists || current.Created.Before(b.Created) {
			latestByNamespace[b.Namespace] = b
		}
	}

	return latestByNamespace
}

// earliestSelectedFullBackup returns the oldest backup from selected per-namespace full backups.
func earliestSelectedFullBackup(selectedFullByNamespace map[string]model.BackupDetails) model.BackupDetails {
	backups := slices.Collect(maps.Values(selectedFullByNamespace))
	return slices.MinFunc(backups, func(a, b model.BackupDetails) int {
		return a.Created.Compare(b.Created)
	})
}

// buildRestoreChainsByNamespace constructs restore chains for each namespace:
// selected full backup first, followed by eligible incrementals in chronological order.
func buildRestoreChainsByNamespace(
	latestFullByNamespace map[string]model.BackupDetails,
	incrementalsByNamespace map[string][]model.BackupDetails,
) map[string][]model.BackupDetails {
	chains := make(map[string][]model.BackupDetails, len(latestFullByNamespace))
	for ns, full := range latestFullByNamespace {
		chains[ns] = append(chains[ns], full)
		for _, incr := range incrementalsByNamespace[ns] {
			// Apply only incrementals strictly newer than the selected full backup.
			if !incr.Created.After(full.Created) {
				continue
			}
			chains[ns] = append(chains[ns], incr)
		}
		slices.SortFunc(chains[ns], func(a, b model.BackupDetails) int {
			return a.Created.Compare(b.Created)
		})
	}

	return chains
}

func splitByNamespace(backups []model.BackupDetails) map[string][]model.BackupDetails {
	backupsByNamespace := make(map[string][]model.BackupDetails)
	for _, b := range backups {
		backupsByNamespace[b.Namespace] = append(backupsByNamespace[b.Namespace], b)
	}

	return backupsByNamespace
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

	client, err := r.clientManager.GetClient(ctx, &request.DestinationCluster, nil, logger)
	if err != nil {
		return fmt.Errorf("failed to get client for cluster %s: %w",
			ptr.ValueOrZero(request.DestinationCluster.ClusterLabel), err)
	}
	defer r.clientManager.Close(client)

	if err := r.validator.ValidateTimestamp(
		ctx,
		request,
		client.InfoClient(),
		backupsByNamespace,
	); err != nil {
		return err
	}

	return r.restoreAllNamespaces(ctx, client, request, jobID, backupsByNamespace, logger)
}

func (r *timeRestoreRunner) restoreAllNamespaces(
	ctx context.Context,
	client aerospike.Client,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	backupsByNamespace map[string][]model.BackupDetails,
	logger *slog.Logger,
) error {
	var (
		wg         sync.WaitGroup
		multiError error
		errMu      sync.Mutex
	)

	// Best-effort restore: process all namespaces even if some fail, then return combined error.
	// This allows users to retry only failed namespaces later.
	// Run namespace restores concurrently and collect errors safely.
	for namespace, nsBackup := range backupsByNamespace {
		wg.Go(func() {
			nsLogger := logger.With(slog.String("namespace", namespace))
			err := r.restoreNamespace(ctx, client, request, jobID, namespace, nsBackup, nsLogger)
			if err != nil {
				errMu.Lock()
				multiError = errors.Join(multiError,
					fmt.Errorf("failed to restore routine %s, namespace %s by timestamp: %w",
						request.Routine.Name, namespace, err))
				errMu.Unlock()
			}
		})
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
	policy, err := prepareNamespaceRestore(
		ctx, client.InfoClient(), namespace, request, backups, logger,
	)
	if err != nil {
		return err
	}

	return r.executeNamespaceRestore(ctx, client, request, jobID, backups, policy, logger)
}

// prepareNamespaceRestore determines the effective restore policy and may reorder
// backups in place via slices.Reverse; the caller passes the same slice to execution.
func prepareNamespaceRestore(
	ctx context.Context,
	infoClient backup.InfoGetter,
	namespace string,
	request *model.RestoreTimestampRequest,
	backups []model.BackupDetails,
	logger *slog.Logger,
) (model.RestorePolicy, error) {
	// Policy is guaranteed non-nil by request validation in the API layer.
	effectivePolicy := request.Policy // make a thread-safe copy.

	if request.DisableReordering {
		return effectivePolicy, nil
	}

	if ptr.ValueOrZero(effectivePolicy.Unique) {
		logger.Info("Use reverse restore order because unique policy is enabled")
		// Restore newest backups first so missing records get the latest version as of the target
		// timestamp; existing records are skipped by the unique (CREATE_ONLY) policy.
		slices.Reverse(backups)
		return effectivePolicy, nil
	}

	counter, err := infoClient.GetRecordCount(ctx, namespace, effectivePolicy.SetList)
	if err != nil {
		return model.RestorePolicy{}, fmt.Errorf(
			"failed to determine if namespace %s is empty: %w", namespace, err,
		)
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

	return effectivePolicy, nil
}

func (r *timeRestoreRunner) executeNamespaceRestore(
	ctx context.Context,
	client aerospike.Restorer,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	backups []model.BackupDetails,
	policy model.RestorePolicy,
	logger *slog.Logger,
) error {
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
		policy.CompressionPolicy = &model.CompressionPolicy{
			Mode: b.Compression,
		}

		handler, err := r.restoreFromPath(ctx, client, request, b.Key, b.Storage, policy)
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
	policy model.RestorePolicy,
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
		return nil, fmt.Errorf("failed to start restore from backup at %s: %w", backupPath, err)
	}

	return handler, nil
}
