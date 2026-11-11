package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/backup-go"
)

// restoreManager restores server backups. It asks the server to run one restore job for each
// namespace. The embedded holder keeps the jobs and answers the status, cancel and listing calls.
type restoreManager struct {
	*service.RestoreJobsHolder

	clientManager aerospike.ClientManager
	catalog       service.BackupReader
	validator     RestoreValidator
}

var _ service.RestoreManager = (*restoreManager)(nil)

// NewRestoreManager returns the RestoreManager of the server backup mode. catalog lists the
// server backups to restore.
func NewRestoreManager(
	clientManager aerospike.ClientManager,
	restoreJobs *service.RestoreJobsHolder,
	catalog service.BackupReader,
	validator RestoreValidator,
) service.RestoreManager {
	return &restoreManager{
		RestoreJobsHolder: restoreJobs,
		clientManager:     clientManager,
		catalog:           catalog,
		validator:         validator,
	}
}

// Restore starts, in the background, the restore of the one backup the request names by its key.
func (m *restoreManager) Restore(request *model.RestoreRequest) model.RestoreJobID {
	jobID, ctx := m.NewJob(request.BackupDataPath)

	logger := slog.With(slog.Any("jobId", jobID))
	logger.Info("New restore job", slog.Any("path", request.BackupDataPath))

	go func() {
		m.FinishJob(jobID, m.restoreBackup(ctx, request, jobID, logger), logger)
	}()

	return jobID
}

func (m *restoreManager) restoreBackup(
	ctx context.Context,
	request *model.RestoreRequest,
	jobID model.RestoreJobID,
	logger *slog.Logger,
) error {
	key := request.BackupDataPath
	backups, err := m.catalog.GetBackups(ctx, service.NewPathFilter(key, request.SourceStorage))
	if err != nil {
		return fmt.Errorf("failed to read server backup %s: %w", key, err)
	}

	client, err := m.clientManager.GetClient(ctx, &request.DestinationCluster, nil, logger)
	if err != nil {
		return err
	}
	defer m.clientManager.Close(client)

	if err := m.validator.ValidatePath(ctx, request, client.InfoClient(), backups); err != nil {
		return err
	}

	nsLogger := logger.With(slog.String("namespace", backups[0].Namespace))

	return m.restoreChain(ctx, client.InfoClient(), jobID, request.Policy, backups, nsLogger)
}

// RestoreByTime starts, in the background, the restore of the routine's namespaces to the
// request's time. The chain of each namespace is restored by one server restore job.
func (m *restoreManager) RestoreByTime(request *model.RestoreTimestampRequest) model.RestoreJobID {
	jobID, ctx := m.NewJob(request.RoutineName)

	logger := slog.With(slog.Any("jobId", jobID))
	logger.Info("New restore by time job",
		slog.String("routine", request.RoutineName),
		slog.Any("timestamp", request.Time))

	go func() {
		m.FinishJob(jobID, m.restoreToTime(ctx, request, jobID, logger), logger)
	}()

	return jobID
}

func (m *restoreManager) restoreToTime(
	ctx context.Context,
	request *model.RestoreTimestampRequest,
	jobID model.RestoreJobID,
	logger *slog.Logger,
) error {
	chains, err := service.FindRestoreChains(ctx, m.catalog, request)
	if err != nil {
		return err
	}

	service.LogChainGaps(chains, logger)

	client, err := m.clientManager.GetClient(ctx, &request.DestinationCluster, nil, logger)
	if err != nil {
		return fmt.Errorf("failed to get client for cluster %s: %w", request.DestinationCluster.Label(), err)
	}
	defer m.clientManager.Close(client)

	err = m.validator.ValidateTimestamp(ctx, request, client.InfoClient(), service.AllBackups(chains))
	if err != nil {
		return err
	}

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs error
	)

	// Every namespace is restored, even when another one fails, so that a retry needs to cover
	// only the failed ones.
	for namespace, chain := range chains {
		wg.Go(func() {
			nsLogger := logger.With(slog.String("namespace", namespace))
			err := m.restoreChain(ctx, client.InfoClient(), jobID, request.Policy, chain.Backups(), nsLogger)
			if err == nil {
				return
			}

			mu.Lock()
			defer mu.Unlock()

			errs = errors.Join(errs, fmt.Errorf("failed to restore routine %s, namespace %s by timestamp: %w",
				request.RoutineName, namespace, err))
		})
	}

	wg.Wait()

	return errs
}

// restoreChain runs one server restore job that applies backups, the backups of one namespace
// oldest first, and waits for it to end.
func (m *restoreManager) restoreChain(
	ctx context.Context,
	infoClient backup.ServerBackupInfo,
	jobID model.RestoreJobID,
	policy model.RestorePolicy,
	backups []model.BackupDetails,
	logger *slog.Logger,
) error {
	job, err := startRestore(ctx, infoClient, policy, backups)
	if err != nil {
		return err
	}

	logger.Info("Server restore started", slog.String("serverJobId", job.jobID))
	m.AddHandler(jobID, job)

	return job.Wait(ctx)
}
