package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/aerospike"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/restoreexecutor"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/aerospike/backup-go"
	"github.com/aerospike/backup-go/models"
	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// maxChainLength is the most backups the server restores in one job (AS_RESTORE_CHAIN_MAX_LINKS).
const maxChainLength = 32

// restoreExecutor asks the server to restore server backups, identified by their job ids. A
// single backup is restored fuzzy, into the live namespace. A full backup and the incremental
// backups that follow it are restored strict (cold): the server takes the namespace from
// clients, drops its data, and hydrates it from the chain.
type restoreExecutor struct {
	backupReader service.BackupReader
}

var _ restoreexecutor.Restore = (*restoreExecutor)(nil)

// NewRestoreExecutor returns a Restore that runs server-side restores. backupReader tells
// which namespace a backup holds, to restore into when the request maps none.
func NewRestoreExecutor(backupReader service.BackupReader) restoreexecutor.Restore {
	return &restoreExecutor{backupReader: backupReader}
}

// Run starts one server restore job that applies the full backup and then each incremental,
// in that order; each request names one backup by its job id in BackupDataPath. The requests
// share their policy and storage, so the full one stands for all.
func (r *restoreExecutor) Run(
	ctx context.Context,
	client aerospike.Client,
	full *model.RestoreRequest,
	incrementals ...*model.RestoreRequest,
) (restoreexecutor.RestoreHandler, error) {
	if len(incrementals)+1 > maxChainLength {
		return nil, fmt.Errorf("the server restores at most %d backups at once, the chain has %d",
			maxChainLength, len(incrementals)+1)
	}

	backupIDs := []string{full.BackupDataPath}
	for _, incremental := range incrementals {
		backupIDs = append(backupIDs, incremental.BackupDataPath)
	}

	return r.start(ctx, client, full, backupIDs)
}

// start starts a server restore job of backupIDs, oldest first, with the policy and storage of
// request.
func (r *restoreExecutor) start(
	ctx context.Context,
	client aerospike.Client,
	request *model.RestoreRequest,
	backupIDs []string,
) (restoreexecutor.RestoreHandler, error) {
	namespace, err := r.destinationNamespace(ctx, request, backupIDs[0])
	if err != nil {
		return nil, err
	}

	common, err := newRequestCommon(request.SourceStorage, namespace)
	if err != nil {
		return nil, err
	}
	common.SetList = strings.Join(request.Policy.SetList, ",")

	jobID := newJobID(time.Now())
	infoClient := client.InfoClient()
	policy := infomodels.RestorePolicyFuzzy
	if len(backupIDs) > 1 {
		policy = infomodels.RestorePolicyStrict
		if err := prepareColdRestore(ctx, infoClient, namespace, jobID); err != nil {
			return nil, err
		}
	}

	// A limit the policy leaves unset is zero, which the request omits: the server's default applies.
	err = infoClient.StartRestore(ctx, &infomodels.RequestRestore{
		RequestCommon:        common,
		JobID:                jobID,
		BackupIDs:            strings.Join(backupIDs, ","),
		RestoreWorkerThreads: ptr.ValueOrZero(request.Policy.Parallel),
		RestoreRPS:           ptr.ValueOrZero(request.Policy.Tps),
		ExtraTTL:             int(ptr.ValueOrZero(request.Policy.ExtraTTL)),
		Policy:               policy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start server restore: %w", err)
	}

	return newRestoreHandler(infoClient, namespace, jobID), nil
}

// prepareColdRestore takes namespace from clients for the cold restore jobID and waits until
// it is empty on every node, clearing the storage of the nodes that still hold data. A failed
// preparation leaves the namespace gated until the job is released with restore-open.
func prepareColdRestore(ctx context.Context, infoClient backup.ServerBackupInfo, namespace, jobID string) error {
	err := infoClient.PrepareRestore(ctx, &infomodels.RequestPrepareRestore{Namespace: namespace, JobID: jobID})
	if err != nil {
		return fmt.Errorf("failed to prepare server restore: %w", err)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		status, err := infoClient.GetRestoreStatus(ctx, namespace, jobID)
		if errors.Is(err, asinfo.ErrNotFound) {
			continue // the job is not visible on the cluster yet.
		}
		if err != nil {
			return fmt.Errorf("failed to get server restore status: %w", err)
		}

		switch {
		case status.NsState == infomodels.NamespaceStateReady:
			return nil
		case status.NsState == infomodels.NamespaceStateFailed:
			return fmt.Errorf("server restore %s failed to prepare namespace %s: %s", jobID, namespace, status.ErrorReason)
		case status.Clear == infomodels.ClearStateRequired:
			if err := infoClient.ClearRestoreStorage(ctx, namespace, jobID); err != nil {
				return fmt.Errorf("failed to clear namespace %s for server restore %s: %w", namespace, jobID, err)
			}
		default:
		}
	}
}

// destinationNamespace returns the namespace to restore into: the mapped destination, or
// else the namespace backupID was taken from.
func (r *restoreExecutor) destinationNamespace(
	ctx context.Context,
	request *model.RestoreRequest,
	backupID string,
) (string, error) {
	if request.Policy.Namespace != nil && request.Policy.Namespace.Destination != "" {
		return request.Policy.Namespace.Destination, nil
	}

	backups, err := r.backupReader.GetBackups(ctx, service.NewPathFilter(backupID, request.SourceStorage))
	if err != nil {
		return "", fmt.Errorf("failed to read server backup %s: %w", backupID, err)
	}
	if len(backups) == 0 {
		return "", fmt.Errorf("server backup %s not found", backupID)
	}

	return backups[0].Namespace, nil
}

// restoreHandler follows a server restore job.
type restoreHandler struct {
	infoClient backup.ServerBackupInfo
	namespace  string
	jobID      string
	stats      *models.RestoreStats
}

var _ restoreexecutor.RestoreHandler = (*restoreHandler)(nil)

func newRestoreHandler(infoClient backup.ServerBackupInfo, namespace, jobID string) *restoreHandler {
	stats := models.NewRestoreStats()
	stats.Start()

	return &restoreHandler{
		infoClient: infoClient,
		namespace:  namespace,
		jobID:      jobID,
		stats:      stats,
	}
}

// Wait polls the restore job until it ends or ctx is canceled.
func (h *restoreHandler) Wait(ctx context.Context) error {
	defer h.stats.Stop()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		status, err := h.infoClient.GetRestoreStatus(ctx, h.namespace, h.jobID)
		if errors.Is(err, asinfo.ErrNotFound) {
			continue // the job is not visible on the cluster yet.
		}
		if err != nil {
			return fmt.Errorf("failed to get server restore status: %w", err)
		}

		switch status.State {
		case infomodels.RestoreStateComplete:
			return nil
		case infomodels.RestoreStateFailed:
			return fmt.Errorf("server restore %s failed: %s", h.jobID, status.ErrorReason)
		case infomodels.RestoreStateAborted:
			return fmt.Errorf("server restore %s was aborted", h.jobID)
		default:
		}
	}
}

// GetStats returns the restore statistics; the server reports no record counts.
func (h *restoreHandler) GetStats() *models.RestoreStats {
	return h.stats
}

// GetMetrics returns empty metrics: the server reports no client-side throughput.
func (h *restoreHandler) GetMetrics() *models.Metrics {
	return &models.Metrics{}
}
