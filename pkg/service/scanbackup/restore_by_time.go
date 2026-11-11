package scanbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
)

// RestoreByTime starts, in the background, the restore of the routine's namespaces to the
// request's time. The namespaces are restored concurrently, the backups of one namespace one
// after another.
func (m *restoreManager) RestoreByTime(request *model.RestoreTimestampRequest) model.RestoreJobID {
	jobID, ctx := m.NewJob(request.RoutineName)

	logger := slog.With(slog.Any("jobId", jobID))
	logger.Info("New restore by time job",
		slog.String("routine", request.RoutineName),
		slog.Any("timestamp", request.Time))

	go func() {
		err := m.restoreToTime(ctx, request, jobID, logger)
		m.FinishJob(jobID, err, logger)
	}()

	return jobID
}

func (m *restoreManager) restoreToTime(
	ctx context.Context,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	logger *slog.Logger,
) error {
	// Lock the routine storage from retention manager for the duration of restore.
	// Restore holds RLock to allow concurrent restores for the same routine.
	routineStorageLock := m.routineStorage.Get(request.RoutineName)
	routineStorageLock.RLock()
	defer routineStorageLock.RUnlock()

	chains, err := service.FindRestoreChains(ctx, m.backupReader, request)
	if err != nil {
		return err
	}

	service.LogChainGaps(chains, logger)

	client, err := m.clientManager.GetClient(ctx, &request.DestinationCluster, nil, logger)
	if err != nil {
		return fmt.Errorf("failed to get client for cluster %s: %w",
			request.DestinationCluster.Label(), err)
	}
	defer m.clientManager.Close(client)

	if err := m.validator.ValidateTimestamp(
		ctx,
		request,
		client.InfoClient(),
		service.AllBackups(chains),
	); err != nil {
		return err
	}

	return m.restoreAllNamespaces(ctx, client, request, jobID, chains, logger)
}

func (m *restoreManager) restoreAllNamespaces(
	ctx context.Context,
	client aerospike.Client,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	chains map[string]service.RestoreChain,
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
	for namespace, chain := range chains {
		wg.Go(func() {
			nsLogger := logger.With(slog.String("namespace", namespace))
			err := m.restoreNamespace(ctx, client, request, jobID, namespace, chain, nsLogger)
			if err != nil {
				errMu.Lock()
				multiError = errors.Join(multiError,
					fmt.Errorf("failed to restore routine %s, namespace %s by timestamp: %w",
						request.RoutineName, namespace, err))
				errMu.Unlock()
			}
		})
	}

	wg.Wait()

	return multiError
}

// restoreNamespace restores the namespace's chain of backups one after another, in the order
// inRestoreOrder gives. Empty backups are left out, and a chain of only empty backups restores
// nothing.
func (m *restoreManager) restoreNamespace(
	ctx context.Context,
	client aerospike.Client,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	namespace string,
	chain service.RestoreChain,
	logger *slog.Logger,
) error {
	backups := chain.Backups()
	requests := make([]*model.RestoreRequest, 0, len(backups))
	for _, b := range backups {
		m.AddTotalRecords(jobID, b.RecordCount)

		if isEmpty(b) { // an empty backup has nothing to restore
			continue
		}

		// For restore-by-time we can extract compression from metadata.
		backupPolicy := request.Policy
		backupPolicy.CompressionPolicy = &model.CompressionPolicy{
			Mode: b.Compression,
		}

		restoreRequest := model.NewRestoreRequest(
			request.DestinationCluster,
			backupPolicy,
			b.Storage,
			request.SecretAgent,
			b.Key,
		)
		restoreRequest.Namespace = namespace
		restoreRequest.DisableReordering = request.DisableReordering
		requests = append(requests, restoreRequest)
	}

	if len(requests) == 0 {
		return nil
	}

	logger.Info("Restoring chain", slog.Int("backups", len(requests)), slog.Any("chain", backups))

	ordered, err := inRestoreOrder(ctx, client, logger, requests)
	if err != nil {
		return err
	}

	return m.restoreInOrder(ctx, client, jobID, ordered, logger)
}

// restoreInOrder restores the backups of the requests one after another and stops at the first
// failure. It logs the counters of the restores added up. A failure names the backup, which the
// restore's own error may not.
func (m *restoreManager) restoreInOrder(
	ctx context.Context,
	client aerospike.Client,
	jobID model.RestoreJobID,
	requests []*model.RestoreRequest,
	logger *slog.Logger,
) error {
	started := time.Now()

	var counters model.RestoreCounters
	for i, request := range requests {
		// A restore can finish just as ctx is canceled. The next one must not start then.
		if err := ctx.Err(); err != nil {
			return err
		}

		path := request.BackupDataPath
		logger.Info("Start restoring",
			slog.String("step", fmt.Sprintf("%d/%d", i+1, len(requests))), slog.String("path", path))

		handler, err := m.executor.Run(ctx, client, request)
		if err != nil {
			return fmt.Errorf("failed to start restore from backup at %s: %w", path, err)
		}

		m.AddHandler(jobID, handler)

		if err := handler.Wait(ctx); err != nil {
			return fmt.Errorf("failed to restore backup at %s: %w", path, err)
		}

		counters = counters.Plus(handler.Status().Counters)
	}

	logger.LogAttrs(ctx, slog.LevelInfo, "Finished restoring", restoreLogAttrs(counters, time.Since(started))...)

	return nil
}
